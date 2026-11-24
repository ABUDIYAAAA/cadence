package tickets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/modules/events"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

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
		t.Skipf("skipping tickets db test: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping tickets db test, db unreachable: %v", err)
	}
	return pool
}

func seedTestEvent(t *testing.T, pool *pgxpool.Pool, slug, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO events (slug, name)
		VALUES ($1, $2)
		RETURNING id
	`, slug, name).Scan(&id); err != nil {
		t.Fatalf("failed to seed test event: %v", err)
	}
	return id
}

func seedTestEventUser(t *testing.T, pool *pgxpool.Pool, eventID, email, name, role string) string {
	t.Helper()
	ctx := context.Background()
	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, global_role, is_active)
		VALUES ($1, $2, 'user', TRUE)
		RETURNING id
	`, email, name).Scan(&userID); err != nil {
		t.Fatalf("failed to seed test user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO user_events (event_id, user_id, role)
		VALUES ($1, $2, $3::event_role)
	`, eventID, userID, role); err != nil {
		t.Fatalf("failed to seed user_events row: %v", err)
	}
	return userID
}

func TestTicketSettingsSanitizationAndExcludedDomains(t *testing.T) {
	pool := newTestDBPool(t)
	defer pool.Close()

	ts := time.Now().UnixNano()
	evSlug := fmt.Sprintf("tkt-settings-ev-%d", ts)
	eventID := seedTestEvent(t, pool, evSlug, "Ticketing Settings Event")
	adminID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("admin_%d@example.com", ts), "Admin", "admin")

	cfg := &config.Config{
		CookieSecret: "test-ticket-cookie-secret-1234567890",
	}
	repo := NewRepository(pool)
	svc := NewService(repo, cfg, nil, nil)

	ctx := events.WithEventContext(context.Background(), &events.Context{
		EventID: eventID,
		Slug:    evSlug,
		Name:    "Ticketing Settings Event",
		UserID:  adminID,
		Role:    "admin",
	})

	subj := "Your Official Pass for {{event_name}}"
	unsafeHTML := `<h1>Hello {{name}}</h1><script>alert('xss')</script><p onload="evil()">Bring your ID to <strong>{{event_name}}</strong>.</p><a href="javascript:steal()">Bad</a>`

	updated, err := svc.UpdateSettings(ctx, UpdateTicketSettingsRequest{
		ExcludedDomains: []string{"@nst.rishihood.edu.in", "staff@internal.edu", "  INTERNAL.EDU "},
		EmailSubject:    &subj,
		EmailHTML:       &unsafeHTML,
	}, adminID, "admin@example.com", "admin", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	if len(updated.ExcludedDomains) != 2 || updated.ExcludedDomains[0] != "internal.edu" || updated.ExcludedDomains[1] != "nst.rishihood.edu.in" {
		t.Fatalf("expected normalized excluded domains [internal.edu nst.rishihood.edu.in], got %v", updated.ExcludedDomains)
	}
	if strings.Contains(updated.EmailHTML, "<script") || strings.Contains(updated.EmailHTML, "onload") || strings.Contains(updated.EmailHTML, "javascript:") {
		t.Fatalf("expected unsafe HTML stripped in saved settings, got: %s", updated.EmailHTML)
	}
	if !strings.Contains(updated.EmailHTML, "<h1>Hello {{name}}</h1>") || !strings.Contains(updated.EmailHTML, "<strong>{{event_name}}</strong>") {
		t.Fatalf("expected safe HTML and placeholders preserved, got: %s", updated.EmailHTML)
	}
}

func TestBulkSendExcludedDomainsHardEmailTrackingAndLateJoinerVisibility(t *testing.T) {
	pool := newTestDBPool(t)
	defer pool.Close()

	ts := time.Now().UnixNano()
	evSlug := fmt.Sprintf("tkt-flow-ev-%d", ts)
	eventID := seedTestEvent(t, pool, evSlug, "Cadence Summit 2026")
	adminID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("admin_flow_%d@example.com", ts), "Admin", "admin")

	// Seed 2 internal users (on excluded @domains) and 2 external users
	internal1 := seedTestEventUser(t, pool, eventID, fmt.Sprintf("student1_%d@nst.rishihood.edu.in", ts), "Internal Student 1", "participant")
	internal2 := seedTestEventUser(t, pool, eventID, fmt.Sprintf("student2_%d@sub.college.edu", ts), "Internal Student 2", "participant")
	external1 := seedTestEventUser(t, pool, eventID, fmt.Sprintf("ext1_%d@gmail.com", ts), "External Guest 1", "participant")
	external2 := seedTestEventUser(t, pool, eventID, fmt.Sprintf("ext2_%d@outlook.com", ts), "External Guest 2", "participant")

	cfg := &config.Config{
		CookieSecret: "test-ticket-cookie-secret-1234567890",
	}
	repo := NewRepository(pool)
	svc := NewService(repo, cfg, nil, nil)

	ctx := events.WithEventContext(context.Background(), &events.Context{
		EventID: eventID,
		Slug:    evSlug,
		Name:    "Cadence Summit 2026",
		UserID:  adminID,
		Role:    "admin",
	})

	// Step 0: Before tickets are released, GetMyTicket returns ErrTicketNotFound
	if _, err := svc.GetMyTicket(ctx, external1); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound before tickets are released, got %v", err)
	}

	// Step 1: Configure excluded domains in settings (@nst.rishihood.edu.in) and custom HTML
	customHTML := `<p>Welcome {{name}} to {{event_name}}!</p><script>evil()</script>`
	if _, err := svc.UpdateSettings(ctx, UpdateTicketSettingsRequest{
		ExcludedDomains: []string{"@nst.rishihood.edu.in"},
		EmailHTML:       &customHTML,
	}, adminID, "admin@example.com", "admin", "127.0.0.1", "test-agent"); err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	// Step 2: Run BulkSendTickets with an additional request-level excluded domain ("@college.edu")
	// Total eligible users in event = 5 (admin + 2 internal + 2 external).
	// Filter to role="participant" so we process the 4 participants.
	partRole := "participant"
	bulkResp1, err := svc.BulkSendTickets(ctx, BulkSendTicketsRequest{
		Role:           &partRole,
		ExcludeDomains: []string{"@college.edu"},
	}, adminID, "admin@example.com", "admin", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("BulkSendTickets run 1 failed: %v", err)
	}

	if bulkResp1.TotalProcessed != 4 {
		t.Fatalf("expected TotalProcessed=4, got %d", bulkResp1.TotalProcessed)
	}
	if bulkResp1.ExcludedDomain != 2 {
		t.Fatalf("expected ExcludedDomain=2 (the 2 internal students), got %d", bulkResp1.ExcludedDomain)
	}
	if bulkResp1.Enqueued != 2 {
		t.Fatalf("expected Enqueued=2 (the 2 external participants), got %d", bulkResp1.Enqueued)
	}
	if bulkResp1.AlreadyMailed != 0 {
		t.Fatalf("expected AlreadyMailed=0 on first run, got %d", bulkResp1.AlreadyMailed)
	}

	// Step 3: Verify excluded-domain internal users STILL have their ticket visible on dashboard, with email_status='skipped_domain'
	for _, uid := range []string{internal1, internal2} {
		myTkt, err := svc.GetMyTicket(ctx, uid)
		if err != nil {
			t.Fatalf("expected excluded-domain participant %s to still see their dashboard ticket, got error: %v", uid, err)
		}
		if myTkt.TicketCode == "" || myTkt.QRCodeBase64 == "" {
			t.Fatalf("expected valid ticket code and QR for internal participant %s", uid)
		}
		if myTkt.EmailStatus != EmailStatusSkippedDomain {
			t.Fatalf("expected EmailStatus=%q for internal user, got %q", EmailStatusSkippedDomain, myTkt.EmailStatus)
		}
	}

	// Step 4: Verify external users have email_status='sent' and EmailSentAt != nil
	for _, uid := range []string{external1, external2} {
		myTkt, err := svc.GetMyTicket(ctx, uid)
		if err != nil {
			t.Fatalf("expected external participant %s to see their ticket: %v", uid, err)
		}
		if myTkt.EmailStatus != EmailStatusSent || myTkt.EmailSentAt == nil {
			t.Fatalf("expected EmailStatus='sent' and non-nil EmailSentAt for external user, got status=%q sentAt=%v", myTkt.EmailStatus, myTkt.EmailSentAt)
		}
	}

	// Step 5: Run BulkSendTickets a second time -> external users must NOT be emailed again (hard tracking)
	bulkResp2, err := svc.BulkSendTickets(ctx, BulkSendTicketsRequest{
		Role:           &partRole,
		ExcludeDomains: []string{"@college.edu"},
	}, adminID, "admin@example.com", "admin", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("BulkSendTickets run 2 failed: %v", err)
	}
	if bulkResp2.Enqueued != 0 {
		t.Fatalf("expected Enqueued=0 on second run (all external users already mailed), got %d", bulkResp2.Enqueued)
	}
	if bulkResp2.AlreadyMailed != 2 {
		t.Fatalf("expected AlreadyMailed=2 on second run, got %d", bulkResp2.AlreadyMailed)
	}
	if bulkResp2.ExcludedDomain != 2 {
		t.Fatalf("expected ExcludedDomain=2 on second run, got %d", bulkResp2.ExcludedDomain)
	}

	// Step 6: Simulate a failed email delivery on external2, then re-run BulkSendTickets -> only external2 is retried!
	tktExt2, err := repo.GetTicketByUserID(ctx, external2)
	if err != nil {
		t.Fatalf("get external2 ticket: %v", err)
	}
	_, _ = pool.Exec(ctx, `UPDATE tickets SET email_status = 'failed', email_sent_at = NULL, email_error = 'smtp timeout' WHERE id = $1`, tktExt2.ID)

	bulkResp3, err := svc.BulkSendTickets(ctx, BulkSendTicketsRequest{
		Role:           &partRole,
		ExcludeDomains: []string{"@college.edu"},
	}, adminID, "admin@example.com", "admin", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("BulkSendTickets run 3 failed: %v", err)
	}
	if bulkResp3.Enqueued != 1 || bulkResp3.AlreadyMailed != 1 {
		t.Fatalf("expected failed user retried (Enqueued=1) and sent user skipped (AlreadyMailed=1), got %+v", bulkResp3)
	}

	// Step 7: ForceResend=true re-emails even already-sent external users (while still respecting excluded domains)
	bulkResp4, err := svc.BulkSendTickets(ctx, BulkSendTicketsRequest{
		Role:           &partRole,
		ExcludeDomains: []string{"@college.edu"},
		ForceResend:    true,
	}, adminID, "admin@example.com", "admin", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("BulkSendTickets run 4 (ForceResend) failed: %v", err)
	}
	if bulkResp4.Enqueued != 2 || bulkResp4.ExcludedDomain != 2 {
		t.Fatalf("expected Enqueued=2 and ExcludedDomain=2 with ForceResend=true, got %+v", bulkResp4)
	}

	// Step 8: Late-joining participant creates their account AFTER tickets were released -> GET /tickets/me works immediately!
	lateParticipantID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("late_user_%d@gmail.com", ts), "Late Joiner", "participant")
	lateTicket, err := svc.GetMyTicket(ctx, lateParticipantID)
	if err != nil {
		t.Fatalf("expected late-joining participant to automatically receive their released ticket on GetMyTicket, got: %v", err)
	}
	if lateTicket.TicketCode == "" || lateTicket.QRCodeBase64 == "" || lateTicket.ReleasedAt == nil {
		t.Fatalf("expected complete auto-minted ticket for late participant, got: %+v", lateTicket)
	}

	// Verify non-member of the event cannot auto-mint a ticket even when event tickets are released
	var outsiderID string
	_ = pool.QueryRow(ctx, `INSERT INTO users (email, name, global_role, is_active) VALUES ($1, 'Outsider', 'user', TRUE) RETURNING id`,
		fmt.Sprintf("outsider_%d@gmail.com", ts)).Scan(&outsiderID)
	if _, err := svc.GetMyTicket(ctx, outsiderID); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected outsider (not in user_events) to receive ErrTicketNotFound, got: %v", err)
	}
}
