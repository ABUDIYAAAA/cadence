package events_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/modules/events"
	"devclubxnst.com/cadence/internal/modules/teams"
	"devclubxnst.com/cadence/internal/modules/users"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func openTestDB(t *testing.T) *pgxpool.Pool {
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
		t.Skipf("skipping db test: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping db test, db unreachable: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func openTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	var rdb *redis.Client
	if opt, err := redis.ParseURL(addr); err == nil {
		rdb = redis.NewClient(opt)
	} else {
		rdb = redis.NewClient(&redis.Options{Addr: addr})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		t.Skipf("skipping redis test, redis unreachable: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func TestBulkAddEventUsersWithCompetitionsAndTeams(t *testing.T) {
	db := openTestDB(t)
	rdb := openTestRedis(t)

	teamsRepo := teams.NewRepository(db)
	usersRepo := users.NewRepository(db).WithRedis(rdb, time.Hour).WithTeamEvictor(teamsRepo)
	repo := events.NewRepository(db).WithUserProvisioner(usersRepo).WithTeamRosterManager(teamsRepo)
	svc := events.NewService(repo, rdb, nil)
	ctx := context.Background()

	nano := time.Now().UnixNano()
	creatorDevID := "00000000-0000-0000-0000-000000000001"
	ownerEmail := fmt.Sprintf("bulk-owner-%d@example.com", nano)
	aliceEmail := fmt.Sprintf("alice-%d@example.com", nano)
	bobEmail := fmt.Sprintf("bob-%d@example.com", nano)
	charlieEmail := fmt.Sprintf("charlie-%d@example.com", nano)
	danaEmail := fmt.Sprintf("dana-%d@example.com", nano)

	ev, err := svc.CreateEvent(ctx, events.CreateEventRequest{
		Slug:       fmt.Sprintf("bulk-event-%d", nano),
		Name:       "Bulk Import Event",
		OwnerEmail: &ownerEmail,
	}, creatorDevID)
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	var ownerID string
	if err := db.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, ownerEmail).Scan(&ownerID); err != nil {
		t.Fatalf("get owner id: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM events WHERE id = $1`, ev.ID)
		_, _ = db.Exec(context.Background(), `DELETE FROM users WHERE email IN ($1, $2, $3, $4, $5)`, ownerEmail, aliceEmail, bobEmail, charlieEmail, danaEmail)
	})

	var aiCompID, webCompID string
	if err := db.QueryRow(ctx, `INSERT INTO competitions (event_id, slug, name, min_team_size, max_team_size) VALUES ($1, 'ai-track', 'AI Track', 1, 2) RETURNING id`, ev.ID).Scan(&aiCompID); err != nil {
		t.Fatalf("create AI Track: %v", err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO competitions (event_id, slug, name, min_team_size, max_team_size) VALUES ($1, 'web-track', 'Web Track', 1, 3) RETURNING id`, ev.ID).Scan(&webCompID); err != nil {
		t.Fatalf("create Web Track: %v", err)
	}

	strPtr := func(s string) *string { return &s }

	resp, err := svc.BulkAddEventUsers(ctx, ev.ID, ownerID, "owner", events.BulkAddEventUsersRequest{
		Users: []events.AddEventUserRequest{
			// Alice in AI Track -> creates account + creates team "Neural Ninjas"
			{Email: aliceEmail, Name: strPtr("Alice"), Role: "participant", CompetitionName: strPtr("AI Track"), TeamName: strPtr("Neural Ninjas")},
			// Bob in AI Track -> creates account + joins "Neural Ninjas" (reaches max_team_size=2)
			{Email: bobEmail, Name: strPtr("Bob"), Role: "participant", CompetitionName: strPtr("AI Track"), TeamName: strPtr("Neural Ninjas")},
			// Alice duplicated in Web Track -> deduplicated user, creates team "Pixel Forge" in Web Track
			{Email: aliceEmail, Name: strPtr("Alice"), Role: "participant", CompetitionName: strPtr("Web Track"), TeamName: strPtr("Pixel Forge")},
			// Exact duplicate of Alice in AI Track -> idempotent, no error
			{Email: aliceEmail, Name: strPtr("Alice"), Role: "participant", CompetitionName: strPtr("AI Track"), TeamName: strPtr("Neural Ninjas")},
			// Charlie tries to join full team "Neural Ninjas" in AI Track -> account created, team skipped
			{Email: charlieEmail, Name: strPtr("Charlie"), Role: "participant", CompetitionName: strPtr("AI Track"), TeamName: strPtr("Neural Ninjas")},
			// Unknown competition -> skipped warning
			{Email: bobEmail, Name: strPtr("Bob"), Role: "participant", CompetitionName: strPtr("Nonexistent"), TeamName: strPtr("Ghost Squad")},
		},
	})
	if err != nil {
		t.Fatalf("BulkAddEventUsers failed: %v", err)
	}

	if resp.TotalProcessed != 6 {
		t.Fatalf("expected TotalProcessed=6, got %d", resp.TotalProcessed)
	}
	if resp.TotalCreated != 3 {
		t.Fatalf("expected TotalCreated=3 unique new accounts, got %d", resp.TotalCreated)
	}
	if len(resp.Users) != 3 {
		t.Fatalf("expected 3 deduplicated users in response, got %d", len(resp.Users))
	}
	if resp.TeamsCreated != 2 {
		t.Fatalf("expected TeamsCreated=2 (Neural Ninjas + Pixel Forge), got %d", resp.TeamsCreated)
	}
	if resp.TeamsAssigned != 3 {
		t.Fatalf("expected TeamsAssigned=3 (Alice+Bob in AI, Alice in Web), got %d", resp.TeamsAssigned)
	}
	if resp.SkippedCount != 2 {
		t.Fatalf("expected SkippedCount=2 (full team + nonexistent comp), got %d (errors: %v)", resp.SkippedCount, resp.SkippedErrors)
	}

	// Verify Alice belongs to 2 teams across the 2 competitions
	var aliceTeamCount int
	if err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE u.email = $1 AND tm.competition_id IN ($2, $3)
	`, aliceEmail, aiCompID, webCompID).Scan(&aliceTeamCount); err != nil {
		t.Fatalf("query alice team count: %v", err)
	}
	if aliceTeamCount != 2 {
		t.Fatalf("expected Alice in 2 teams across competitions, got %d", aliceTeamCount)
	}

	// Freeze "Pixel Forge" and verify bulk team assignment respects team freeze rules via teams.RosterManager
	if _, err := db.Exec(ctx, `UPDATE teams SET is_frozen = TRUE WHERE competition_id = $1 AND name = 'Pixel Forge'`, webCompID); err != nil {
		t.Fatalf("freeze Pixel Forge: %v", err)
	}
	frozenResp, err := svc.BulkAddEventUsers(ctx, ev.ID, ownerID, "owner", events.BulkAddEventUsersRequest{
		Users: []events.AddEventUserRequest{
			{Email: danaEmail, Name: strPtr("Dana"), Role: "participant", CompetitionName: strPtr("Web Track"), TeamName: strPtr("Pixel Forge")},
		},
	})
	if err != nil {
		t.Fatalf("BulkAddEventUsers frozen check failed: %v", err)
	}
	if frozenResp.TeamsAssigned != 0 || frozenResp.SkippedCount != 1 {
		t.Fatalf("expected frozen team assignment to be rejected (assigned=0, skipped=1), got assigned=%d skipped=%d errors=%v",
			frozenResp.TeamsAssigned, frozenResp.SkippedCount, frozenResp.SkippedErrors)
	}
}
