package notifications

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/modules/events"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func TestValidatePushEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid FCM https endpoint",
			url:     "https://fcm.googleapis.com/fcm/send/sample-token",
			wantErr: false,
		},
		{
			name:    "valid Mozilla push endpoint",
			url:     "https://updates.push.services.mozilla.com/wpush/v2/sample-token",
			wantErr: false,
		},
		{
			name:    "insecure http scheme",
			url:     "http://fcm.googleapis.com/fcm/send/sample-token",
			wantErr: true,
		},
		{
			name:    "loopback IPv4 address",
			url:     "https://127.0.0.1/notify",
			wantErr: true,
		},
		{
			name:    "loopback localhost hostname",
			url:     "https://localhost/notify",
			wantErr: true,
		},
		{
			name:    "cloud metadata link-local IP",
			url:     "https://169.254.169.254/latest/meta-data/",
			wantErr: true,
		},
		{
			name:    "private RFC 1918 10.0.0.0/8",
			url:     "https://10.0.1.5:443/push",
			wantErr: true,
		},
		{
			name:    "private RFC 1918 172.16.0.0/12",
			url:     "https://172.20.0.2/push",
			wantErr: true,
		},
		{
			name:    "private RFC 1918 192.168.0.0/16",
			url:     "https://192.168.1.100/push",
			wantErr: true,
		},
		{
			name:    "custom port probe",
			url:     "https://fcm.googleapis.com:8080/push",
			wantErr: true,
		},
		{
			name:    "internal service custom port",
			url:     "https://127.0.0.1:15672/api",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePushEndpoint(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePushEndpoint(%q) error = %v, wantErr = %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestIsRestrictedIP(t *testing.T) {
	tests := []struct {
		name       string
		ip         string
		restricted bool
	}{
		{"IPv4 Loopback", "127.0.0.1", true},
		{"IPv6 Loopback", "::1", true},
		{"Cloud metadata", "169.254.169.254", true},
		{"Link-local unicast", "169.254.1.1", true},
		{"Private class A", "10.10.10.10", true},
		{"Private class B", "172.16.1.1", true},
		{"Private class C", "192.168.1.1", true},
		{"Carrier grade NAT", "100.64.0.1", true},
		{"Unspecified IPv4", "0.0.0.0", true},
		{"Unspecified IPv6", "::", true},
		{"Public Cloudflare DNS", "1.1.1.1", false},
		{"Public Google DNS", "8.8.8.8", false},
		{"Public Quad9 DNS", "9.9.9.9", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := net.ParseIP(tt.ip)
			if parsed == nil {
				t.Fatalf("failed to parse IP %s", tt.ip)
			}
			got := IsRestrictedIP(parsed)
			if got != tt.restricted {
				t.Errorf("IsRestrictedIP(%s) = %v, want %v", tt.ip, got, tt.restricted)
			}
		})
	}
}

func newTestDBPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:password@127.0.0.1:5434/lms?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping notifications db test: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping notifications db test, db unreachable: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})
	return pool
}

func TestNotificationTargetingUserTeamCompetitionAndEvent(t *testing.T) {
	pool := newTestDBPool(t)

	ctx := context.Background()
	ts := time.Now().UnixNano()
	evSlug := fmt.Sprintf("notif-ev-%d", ts)
	logo := "https://cdn.example.com/events/summit-logo.png"

	var eventID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO events (slug, name, logo_url)
		VALUES ($1, 'Notification Test Event', $2)
		RETURNING id
	`, evSlug, logo).Scan(&eventID); err != nil {
		t.Fatalf("seed event: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM events WHERE id = $1`, eventID)
	})

	seedUser := func(prefix, role string) string {
		var uid string
		if err := pool.QueryRow(ctx, `
			INSERT INTO users (email, name, role, is_active)
			VALUES ($1, $2, 'user', TRUE)
			RETURNING id
		`, fmt.Sprintf("%s_%d@example.com", prefix, ts), prefix).Scan(&uid); err != nil {
			t.Fatalf("seed user: %v", err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, uid)
		})
		if _, err := pool.Exec(ctx, `
			INSERT INTO user_events (event_id, user_id, role)
			VALUES ($1, $2, $3::event_role)
		`, eventID, uid, role); err != nil {
			t.Fatalf("seed user_event: %v", err)
		}
		return uid
	}

	adminID := seedUser("notif_admin", "admin")
	u1 := seedUser("notif_u1", "participant")
	u2 := seedUser("notif_u2", "participant")
	u3 := seedUser("notif_u3", "participant")

	var compAID, compBID string
	_ = pool.QueryRow(ctx, `INSERT INTO competitions (event_id, slug, name) VALUES ($1, $2, 'Comp A') RETURNING id`, eventID, fmt.Sprintf("comp-a-%d", ts)).Scan(&compAID)
	_ = pool.QueryRow(ctx, `INSERT INTO competitions (event_id, slug, name) VALUES ($1, $2, 'Comp B') RETURNING id`, eventID, fmt.Sprintf("comp-b-%d", ts)).Scan(&compBID)

	var teamA1, teamB1 string
	_ = pool.QueryRow(ctx, `INSERT INTO teams (competition_id, name, join_code, leader_id, created_by) VALUES ($1, $2, $3, $4, $4) RETURNING id`,
		compAID, fmt.Sprintf("Team A1 %d", ts), fmt.Sprintf("TA%05d", ts%100000), u1).Scan(&teamA1)
	_ = pool.QueryRow(ctx, `INSERT INTO teams (competition_id, name, join_code, leader_id, created_by) VALUES ($1, $2, $3, $4, $4) RETURNING id`,
		compBID, fmt.Sprintf("Team B1 %d", ts), fmt.Sprintf("TB%05d", ts%100000), u3).Scan(&teamB1)

	// u1 and u2 are in Team A1 (Comp A); u3 is in Team B1 (Comp B)
	_, _ = pool.Exec(ctx, `INSERT INTO team_members (team_id, user_id, competition_id) VALUES ($1, $2, $3), ($1, $4, $3)`, teamA1, u1, compAID, u2)
	_, _ = pool.Exec(ctx, `INSERT INTO team_members (team_id, user_id, competition_id) VALUES ($1, $2, $3)`, teamB1, u3, compBID)

	evCtx := events.WithEvent(ctx, &events.Event{
		ID:      eventID,
		Slug:    evSlug,
		Name:    "Notification Test Event",
		LogoURL: &logo,
	})
	evCtx = events.WithEventContext(evCtx, &events.Context{
		EventID: eventID,
		Slug:    evSlug,
		Name:    "Notification Test Event",
		UserID:  adminID,
		Role:    "admin",
	})

	repo := NewRepository(pool)
	svc := NewService(repo, nil, &config.Config{}, nil)

	// 1. Target single user (u1)
	resUser, err := svc.Broadcast(evCtx, BroadcastNotificationRequest{
		Title:      "Direct User Notice",
		Body:       "Hello u1",
		TargetType: "user",
		TargetID:   &u1,
	}, adminID, "admin@example.com", "admin", "127.0.0.1", "test")
	if err != nil || resUser.RecipientsCount != 1 {
		t.Fatalf("expected 1 recipient for user target, got %+v (err: %v)", resUser, err)
	}

	// 2. Target team (teamA1 -> u1, u2)
	resTeam, err := svc.Broadcast(evCtx, BroadcastNotificationRequest{
		Title:      "Team Notice",
		Body:       "Hello Team A1",
		TargetType: "team",
		TargetID:   &teamA1,
	}, adminID, "admin@example.com", "admin", "127.0.0.1", "test")
	if err != nil || resTeam.RecipientsCount != 2 {
		t.Fatalf("expected 2 recipients for team target, got %+v (err: %v)", resTeam, err)
	}

	// 3. Target competition (compAID -> u1, u2)
	resComp, err := svc.Broadcast(evCtx, BroadcastNotificationRequest{
		Title:      "Competition Notice",
		Body:       "Hello Comp A",
		TargetType: "competition",
		TargetID:   &compAID,
	}, adminID, "admin@example.com", "admin", "127.0.0.1", "test")
	if err != nil || resComp.RecipientsCount != 2 {
		t.Fatalf("expected 2 recipients for competition target, got %+v (err: %v)", resComp, err)
	}

	// 4. Target whole event ("event" -> admin + u1 + u2 + u3 = 4)
	resEvent, err := svc.Broadcast(evCtx, BroadcastNotificationRequest{
		Title:      "Whole Event Notice",
		Body:       "Hello Everyone",
		TargetType: "event",
	}, adminID, "admin@example.com", "admin", "127.0.0.1", "test")
	if err != nil || resEvent.RecipientsCount != 4 {
		t.Fatalf("expected 4 recipients for whole event target, got %+v (err: %v)", resEvent, err)
	}

	// Verify u1 has 4 notifications in inbox and default IconURL was populated from event LogoURL
	inboxU1, err := svc.GetInbox(evCtx, u1, 1, 20)
	if err != nil || inboxU1.TotalCount != 4 {
		t.Fatalf("expected 4 notifications in u1 inbox, got %+v (err: %v)", inboxU1, err)
	}
	if inboxU1.Notifications[0].IconURL == nil || *inboxU1.Notifications[0].IconURL != logo {
		t.Fatalf("expected notification IconURL to default to event LogoURL %q, got %v", logo, inboxU1.Notifications[0].IconURL)
	}
}
