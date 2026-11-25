package competitions

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/internal/modules/events"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func TestRequireCompetitionRoles(t *testing.T) {
	adminOrOwnerOnly := RequireCompetitionRoles("owner", "admin")

	testHandler := adminOrOwnerOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compCtx, ok := FromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(compCtx.Role + ":" + compCtx.CompetitionID))
	}))

	t.Run("missing competition context returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rec.Code)
		}
	})

	t.Run("owner role allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithCompetitionContext(req.Context(), &Context{
			CompetitionID: "comp-1",
			EventID:       "ev-1",
			Slug:          "hack-2026",
			Name:          "Hack 2026",
			UserID:        "user-owner",
			Role:          "owner",
		})
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req.WithContext(ctx))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if rec.Body.String() != "owner:comp-1" {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("admin role allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithCompetitionContext(req.Context(), &Context{
			CompetitionID: "comp-1",
			EventID:       "ev-1",
			Slug:          "hack-2026",
			Name:          "Hack 2026",
			UserID:        "user-admin",
			Role:          "admin",
		})
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req.WithContext(ctx))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("judge role rejected with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithCompetitionContext(req.Context(), &Context{
			CompetitionID: "comp-1",
			EventID:       "ev-1",
			Slug:          "hack-2026",
			Name:          "Hack 2026",
			UserID:        "user-judge",
			Role:          "judge",
		})
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req.WithContext(ctx))

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rec.Code)
		}
	})

	t.Run("participant role rejected with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithCompetitionContext(req.Context(), &Context{
			CompetitionID: "comp-1",
			EventID:       "ev-1",
			Slug:          "hack-2026",
			Name:          "Hack 2026",
			UserID:        "user-part",
			Role:          "participant",
		})
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req.WithContext(ctx))

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rec.Code)
		}
	})
}

func TestSlugRegexValidation(t *testing.T) {
	validSlugs := []string{
		"hack-2026",
		"devclub-hackathon",
		"ai-challenge-1",
		"delhi-hacks",
		"h1",
	}

	for _, s := range validSlugs {
		if !slugRegex.MatchString(s) {
			t.Errorf("expected slug %q to be valid", s)
		}
	}

	invalidSlugs := []string{
		"Hack-2026",  // uppercase
		"hack_2026",  // underscore
		"-hack",      // leading hyphen
		"hack-",      // trailing hyphen
		"hack--2026", // consecutive hyphens
		"hack 2026",  // space
		"hack.2026",  // period
	}

	for _, s := range invalidSlugs {
		if slugRegex.MatchString(s) {
			t.Errorf("expected slug %q to be invalid", s)
		}
	}
}

func intPtr(v int) *int { return &v }

func TestValidationDTOs(t *testing.T) {
	t.Run("valid CreateCompetitionRequest passes", func(t *testing.T) {
		req := CreateCompetitionRequest{
			Slug:        "hack-2026",
			Name:        "Hack 2026",
			MinTeamSize: intPtr(2),
			MaxTeamSize: intPtr(4),
		}
		errs := utils.ValidateStruct(req)
		if len(errs) > 0 {
			t.Fatalf("unexpected validation errors: %v", errs)
		}
	})

	t.Run("missing min or max team size fails", func(t *testing.T) {
		req := CreateCompetitionRequest{
			Slug: "hack-2026",
			Name: "Hack 2026",
		}
		errs := utils.ValidateStruct(req)
		if len(errs) == 0 {
			t.Fatalf("expected validation error when min_team_size and max_team_size are omitted")
		}
	})

	t.Run("invalid slug too short fails", func(t *testing.T) {
		req := CreateCompetitionRequest{
			Slug:        "a",
			Name:        "Hack 2026",
			MinTeamSize: intPtr(1),
			MaxTeamSize: intPtr(4),
		}
		errs := utils.ValidateStruct(req)
		if len(errs) == 0 {
			t.Fatalf("expected validation error for short slug")
		}
	})
}

func TestEventScopedCompetitionCreationPermissions(t *testing.T) {
	r := chi.NewRouter()
	handler := NewHandler(&Service{})

	RegisterEventScopedRoutes(r, handler, nil, nil)

	makeReq := func(role string) *httptest.ResponseRecorder {
		body := bytes.NewBufferString(`{invalid-json`)
		req := httptest.NewRequest(http.MethodPost, "/competitions", body)
		req.Header.Set("Content-Type", "application/json")

		ctx := req.Context()
		ctx = auth.WithUserClaims(ctx, &utils.AccessTokenClaims{
			UserID: "user-1",
			Email:  "user1@cadence.local",
			Role:   "user",
		})
		if role != "" {
			ctx = events.WithEventContext(ctx, &events.Context{
				EventID: "ev-1",
				Slug:    "fest-2026",
				Name:    "Fest 2026",
				UserID:  "user-1",
				Role:    role,
			})
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req.WithContext(ctx))
		return rec
	}

	t.Run("owner passes RequireEventRoles and reaches handler", func(t *testing.T) {
		rec := makeReq("owner")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request (passed role check), got %d", rec.Code)
		}
	})

	t.Run("admin passes RequireEventRoles and reaches handler", func(t *testing.T) {
		rec := makeReq("admin")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request (passed role check), got %d", rec.Code)
		}
	})

	t.Run("volunteer is rejected with 403 Forbidden", func(t *testing.T) {
		rec := makeReq("volunteer")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for volunteer, got %d", rec.Code)
		}
	})

	t.Run("judge is rejected with 403 Forbidden", func(t *testing.T) {
		rec := makeReq("judge")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for judge, got %d", rec.Code)
		}
	})

	t.Run("participant is rejected with 403 Forbidden", func(t *testing.T) {
		rec := makeReq("participant")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for participant, got %d", rec.Code)
		}
	})
}

func newTestDBPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_ = godotenv.Load("../../.env")
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
		t.Skipf("skipping db test: ping failed: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})
	return pool
}

func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	opts, err := redis.ParseURL("redis://:password@127.0.0.1:6379/15")
	if err != nil {
		t.Skip("skipping Redis test: invalid redis options")
	}
	rdb := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping Redis test: redis not reachable: %v", err)
	}
	t.Cleanup(func() {
		_ = rdb.Close()
	})
	return rdb
}

func TestCompetitionCRUDUnderEvents(t *testing.T) {
	db := newTestDBPool(t)
	rdb := newTestRedisClient(t)

	eventsRepo := events.NewRepository(db)
	eventsSvc := events.NewService(eventsRepo, rdb, nil)

	compRepo := NewRepository(db)
	compSvc := NewService(compRepo, rdb, nil)
	ctx := context.Background()

	nano := time.Now().UnixNano()
	creatorDevID := "00000000-0000-0000-0000-000000000001"
	ownerAEmail := fmt.Sprintf("owner-a-%d@example.com", nano)
	ownerBEmail := fmt.Sprintf("owner-b-%d@example.com", nano)

	evA, err := eventsSvc.CreateEvent(ctx, events.CreateEventRequest{
		Slug:       fmt.Sprintf("event-a-%d", nano),
		Name:       "Event A",
		OwnerEmail: &ownerAEmail,
	}, creatorDevID)
	if err != nil {
		t.Fatalf("failed to create event A: %v", err)
	}

	evB, err := eventsSvc.CreateEvent(ctx, events.CreateEventRequest{
		Slug:       fmt.Sprintf("event-b-%d", nano),
		Name:       "Event B",
		OwnerEmail: &ownerBEmail,
	}, creatorDevID)
	if err != nil {
		t.Fatalf("failed to create event B: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM events WHERE id IN ($1, $2)`, evA.ID, evB.ID)
		_, _ = db.Exec(context.Background(), `DELETE FROM users WHERE email IN ($1, $2)`, ownerAEmail, ownerBEmail)
	})

	sharedCompSlug := fmt.Sprintf("shared-comp-%d", nano)

	t.Run("create competition in Event A and same slug in Event B succeeds, duplicate in Event A fails", func(t *testing.T) {
		compA, err := compSvc.CreateCompetition(ctx, evA.ID, CreateCompetitionRequest{
			Slug:        sharedCompSlug,
			Name:        "Hackathon in Event A",
			MinTeamSize: intPtr(2),
			MaxTeamSize: intPtr(4),
		}, creatorDevID)
		if err != nil {
			t.Fatalf("failed to create competition in event A: %v", err)
		}
		if compA.EventID != evA.ID {
			t.Fatalf("expected event_id %s, got %s", evA.ID, compA.EventID)
		}
		if compA.MinTeamSize != 2 || compA.MaxTeamSize != 4 {
			t.Fatalf("expected team size bounds 2..4, got %d..%d", compA.MinTeamSize, compA.MaxTeamSize)
		}

		// Duplicate slug inside the same event must fail
		_, err = compSvc.CreateCompetition(ctx, evA.ID, CreateCompetitionRequest{
			Slug:        sharedCompSlug,
			Name:        "Duplicate in Event A",
			MinTeamSize: intPtr(1),
			MaxTeamSize: intPtr(4),
		}, creatorDevID)
		if err != ErrSlugAlreadyExists {
			t.Fatalf("expected ErrSlugAlreadyExists in same event, got %v", err)
		}

		// Invalid range (min > max) must fail
		_, err = compSvc.CreateCompetition(ctx, evA.ID, CreateCompetitionRequest{
			Slug:        sharedCompSlug + "-bad-range",
			Name:        "Bad Range",
			MinTeamSize: intPtr(5),
			MaxTeamSize: intPtr(3),
		}, creatorDevID)
		if err != ErrInvalidTeamSizeRange {
			t.Fatalf("expected ErrInvalidTeamSizeRange, got %v", err)
		}

		// Same slug inside Event B must succeed
		compB, err := compSvc.CreateCompetition(ctx, evB.ID, CreateCompetitionRequest{
			Slug:        sharedCompSlug,
			Name:        "Hackathon in Event B",
			MinTeamSize: intPtr(1),
			MaxTeamSize: intPtr(5),
		}, creatorDevID)
		if err != nil {
			t.Fatalf("expected same slug in Event B to succeed, got %v", err)
		}
		if compB.EventID != evB.ID {
			t.Fatalf("expected event_id %s, got %s", evB.ID, compB.EventID)
		}

		// Update competition in Event A
		newName := "Updated Hackathon A"
		updated, err := compSvc.UpdateCompetition(ctx, evA.ID, compA.ID, sharedCompSlug, UpdateCompetitionRequest{
			Name: &newName,
		}, creatorDevID)
		if err != nil || updated.Name != newName {
			t.Fatalf("failed to update competition: updated=%+v err=%v", updated, err)
		}

		// Delete competition in Event B
		if err := compSvc.DeleteCompetition(ctx, evB.ID, compB.ID, creatorDevID); err != nil {
			t.Fatalf("failed to delete competition: %v", err)
		}
		listB, err := compSvc.ListCompetitionsByEvent(ctx, evB.ID)
		if err != nil || len(listB) != 0 {
			t.Fatalf("expected 0 competitions in Event B after delete, got %d (err: %v)", len(listB), err)
		}

		// --- Competition member management ---
		var ownerAID string
		if err := db.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, ownerAEmail).Scan(&ownerAID); err != nil {
			t.Fatalf("failed to load owner A id: %v", err)
		}
		memberTS := time.Now().UnixNano()
		aliceName := "Alice Comp"
		aliceEmail := fmt.Sprintf("alice-comp-%d@example.com", memberTS)
		bobName := "Bob Comp"
		bobEmail := fmt.Sprintf("bob-comp-%d@example.com", memberTS)
		teamAlpha := "Alpha Squad"

		// Volunteer cannot add a judge
		if _, err := compSvc.AddCompetitionUser(ctx, evA.ID, compA.ID, ownerAID, "volunteer", events.AddEventUserRequest{
			Email: aliceEmail,
			Name:  &aliceName,
			Role:  "judge",
		}); err != ErrPermissionDenied {
			t.Fatalf("expected volunteer adding judge to fail with ErrPermissionDenied, got %v", err)
		}

		// Owner adds Alice as participant with team assignment
		alice, err := compSvc.AddCompetitionUser(ctx, evA.ID, compA.ID, ownerAID, "owner", events.AddEventUserRequest{
			Email:    aliceEmail,
			Name:     &aliceName,
			Role:     "participant",
			TeamName: &teamAlpha,
		})
		if err != nil {
			t.Fatalf("failed to add Alice to competition: %v", err)
		}
		defer func() {
			_, _ = db.Exec(ctx, `DELETE FROM users WHERE email IN ($1, $2)`, aliceEmail, bobEmail)
		}()

		// Verify Alice was auto-enrolled in parent event
		var inEvent bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_events WHERE event_id = $1 AND user_id = $2)`, evA.ID, alice.ID).Scan(&inEvent); err != nil || !inEvent {
			t.Fatalf("expected Alice to be auto-enrolled in parent event, inEvent=%v err=%v", inEvent, err)
		}

		// List competition users returns Alice with her team
		listed, err := compSvc.ListCompetitionUsers(ctx, evA.ID, compA.ID, events.EventUserFilter{Page: 1, Limit: 25})
		if err != nil || listed.Total != 1 || listed.Users[0].TeamName == nil || *listed.Users[0].TeamName != teamAlpha {
			t.Fatalf("unexpected listed competition users: %+v err=%v", listed, err)
		}

		// Update Alice's competition role to judge
		if err := compSvc.UpdateCompetitionMemberRole(ctx, evA.ID, compA.ID, ownerAID, "owner", alice.ID, "judge"); err != nil {
			t.Fatalf("failed to update Alice competition role: %v", err)
		}

		// Ban Alice -> marks banned and evicts from competition team
		if err := compSvc.BanCompetitionMember(ctx, evA.ID, compA.ID, ownerAID, "owner", alice.ID, nil); err != nil {
			t.Fatalf("failed to ban Alice in competition: %v", err)
		}
		var teamMembershipCount int
		if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM team_members WHERE competition_id = $1 AND user_id = $2`, compA.ID, alice.ID).Scan(&teamMembershipCount); err != nil || teamMembershipCount != 0 {
			t.Fatalf("expected Alice evicted from competition team on ban, count=%d err=%v", teamMembershipCount, err)
		}

		// Unban Alice
		if err := compSvc.UnbanCompetitionMember(ctx, evA.ID, compA.ID, ownerAID, alice.ID); err != nil {
			t.Fatalf("failed to unban Alice: %v", err)
		}

		// Bulk import Bob into competition
		bulkResp, err := compSvc.BulkAddCompetitionUsers(ctx, evA.ID, compA.ID, ownerAID, "owner", events.BulkAddEventUsersRequest{
			Users: []events.AddEventUserRequest{
				{Email: bobEmail, Name: &bobName, Role: "participant", TeamName: &teamAlpha},
			},
		})
		if err != nil || bulkResp.TotalCreated != 1 || bulkResp.TeamsAssigned != 1 {
			t.Fatalf("unexpected bulk import response: %+v err=%v", bulkResp, err)
		}

		// Remove Alice from competition (still remains in parent event)
		if err := compSvc.RemoveCompetitionMember(ctx, evA.ID, compA.ID, ownerAID, "owner", alice.ID); err != nil {
			t.Fatalf("failed to remove Alice from competition: %v", err)
		}
		listedAfterRemove, err := compSvc.ListCompetitionUsers(ctx, evA.ID, compA.ID, events.EventUserFilter{Page: 1, Limit: 25})
		if err != nil || listedAfterRemove.Total != 1 || listedAfterRemove.Users[0].Email != bobEmail {
			t.Fatalf("expected only Bob after removing Alice, got %+v err=%v", listedAfterRemove, err)
		}
	})
}
