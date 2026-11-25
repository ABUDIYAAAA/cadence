package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func TestRequireEventRoles(t *testing.T) {
	adminOrOwnerOnly := RequireEventRoles("owner", "admin")

	testHandler := adminOrOwnerOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		evCtx, ok := FromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(evCtx.Role + ":" + evCtx.EventID))
	}))

	t.Run("missing event context returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rec.Code)
		}
	})

	t.Run("owner role allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithEventContext(req.Context(), &Context{
			EventID: "ev-1",
			Slug:    "fest-2026",
			Name:    "Fest 2026",
			UserID:  "user-owner",
			Role:    "owner",
		})
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req.WithContext(ctx))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if rec.Body.String() != "owner:ev-1" {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("admin role allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithEventContext(req.Context(), &Context{
			EventID: "ev-1",
			Slug:    "fest-2026",
			Name:    "Fest 2026",
			UserID:  "user-admin",
			Role:    "admin",
		})
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req.WithContext(ctx))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("judge role rejected with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithEventContext(req.Context(), &Context{
			EventID: "ev-1",
			Slug:    "fest-2026",
			Name:    "Fest 2026",
			UserID:  "user-judge",
			Role:    "judge",
		})
		rec := httptest.NewRecorder()
		testHandler.ServeHTTP(rec, req.WithContext(ctx))

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rec.Code)
		}
	})

	t.Run("participant role rejected with 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithEventContext(req.Context(), &Context{
			EventID: "ev-1",
			Slug:    "fest-2026",
			Name:    "Fest 2026",
			UserID:  "user-part",
			Role:    "participant",
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
		"fest-2026",
		"devclub-event",
		"ai-summit-1",
		"delhi-hacks",
		"e1",
	}

	for _, s := range validSlugs {
		if !slugRegex.MatchString(s) {
			t.Errorf("expected slug %q to be valid", s)
		}
	}

	invalidSlugs := []string{
		"Fest-2026",  // uppercase
		"fest_2026",  // underscore
		"-fest",      // leading hyphen
		"fest-",      // trailing hyphen
		"fest--2026", // consecutive hyphens
		"fest 2026",  // space
		"fest.2026",  // period
	}

	for _, s := range invalidSlugs {
		if slugRegex.MatchString(s) {
			t.Errorf("expected slug %q to be invalid", s)
		}
	}
}

func TestRoleHierarchyRules(t *testing.T) {
	svc := &Service{}

	t.Run("admin cannot assign admin role", func(t *testing.T) {
		err := svc.AssignOrUpdateMemberRole(context.Background(), "ev-1", "actor-admin", "admin", "target-user", "admin")
		if err != ErrPermissionDenied {
			t.Fatalf("expected ErrPermissionDenied, got %v", err)
		}
	})

	t.Run("owner cannot assign owner role via update endpoint", func(t *testing.T) {
		err := svc.AssignOrUpdateMemberRole(context.Background(), "ev-1", "actor-owner", "owner", "target-user", "owner")
		if err == nil || err.Error() != "cannot assign owner role; use transfer ownership endpoint" {
			t.Fatalf("expected cannot assign owner role error, got %v", err)
		}
	})

	t.Run("owner cannot self-demote via role update", func(t *testing.T) {
		err := svc.AssignOrUpdateMemberRole(context.Background(), "ev-1", "actor-owner", "owner", "actor-owner", "participant")
		if err != ErrCannotSelfDemote {
			t.Fatalf("expected ErrCannotSelfDemote, got %v", err)
		}
	})
}

func TestCreateAndDeleteEventDevOnlyRestriction(t *testing.T) {
	r := chi.NewRouter()
	handler := NewHandler(&Service{})

	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}

	RegisterRoutes(r, handler, nil, nil, authMiddleware)

	t.Run("unauthenticated user is rejected with 401 by RequireDev", func(t *testing.T) {
		body := bytes.NewBufferString(`{"slug":"fest-2026","name":"Fest 2026"}`)
		req := httptest.NewRequest(http.MethodPost, "/events", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("regular user with role=user is rejected with 403 on create event", func(t *testing.T) {
		body := bytes.NewBufferString(`{"slug":"fest-2026","name":"Fest 2026"}`)
		req := httptest.NewRequest(http.MethodPost, "/events", body)
		req.Header.Set("Content-Type", "application/json")

		claims := &utils.AccessTokenClaims{
			UserID: "user-regular-1",
			Email:  "regular@cadence.local",
			Role:   "user",
		}
		ctx := auth.WithUserClaims(req.Context(), claims)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req.WithContext(ctx))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for role=user, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("dev user with role=dev passes RequireDev gate", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid-json`)
		req := httptest.NewRequest(http.MethodPost, "/events", body)
		req.Header.Set("Content-Type", "application/json")

		claims := &utils.AccessTokenClaims{
			UserID: "user-dev-1",
			Email:  "dev@cadence.local",
			Role:   "dev",
		}
		ctx := auth.WithUserClaims(req.Context(), claims)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req.WithContext(ctx))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request (passed dev check), got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})
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
	return rdb
}

func TestRequireMembershipBanEnforcement(t *testing.T) {
	t.Run("RequireMembership rejects banned user with 403 via Redis cache", func(t *testing.T) {
		rdb := newTestRedisClient(t)
		defer rdb.Close()
		ctx := context.Background()
		_ = rdb.Set(ctx, "event:ev-1:member:user-banned-1", "banned", time.Minute).Err()

		mw := RequireMembership(nil, rdb)
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		claims := &utils.AccessTokenClaims{
			UserID: "user-banned-1",
			Email:  "banned@cadence.local",
			Role:   "user",
		}
		ev := &Event{
			ID:   "ev-1",
			Slug: "fest-2026",
			Name: "Fest 2026",
		}

		reqCtx := auth.WithUserClaims(ctx, claims)
		reqCtx = WithEvent(reqCtx, ev)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(reqCtx)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for banned member, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["error"] != "you are banned from this event" {
			t.Fatalf("expected 'you are banned from this event', got %v", resp["error"])
		}
	})

	t.Run("Dev bypass grants owner in RequireMembership", func(t *testing.T) {
		mw := RequireMembership(nil, nil)
		var capturedRole string
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			evCtx, ok := FromContext(r.Context())
			if ok {
				capturedRole = evCtx.Role
			}
			w.WriteHeader(http.StatusOK)
		}))

		claims := &utils.AccessTokenClaims{
			UserID: "user-dev-1",
			Email:  "dev@cadence.local",
			Role:   "dev",
		}
		ev := &Event{
			ID:   "ev-1",
			Slug: "fest-2026",
			Name: "Fest 2026",
		}

		ctx := auth.WithUserClaims(context.Background(), claims)
		ctx = WithEvent(ctx, ev)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for dev, got %d", rec.Code)
		}
		if capturedRole != "owner" {
			t.Fatalf("expected role 'owner' for dev, got %s", capturedRole)
		}
	})

	t.Run("RequireMembership keeps AccessTokenClaims.Role immutable and exposes tenant role via auth.GetEffectiveRole", func(t *testing.T) {
		rdb := newTestRedisClient(t)
		defer rdb.Close()
		ctx := context.Background()
		_ = rdb.Set(ctx, "event:ev-1:member:user-admin-1", "admin", time.Minute).Err()

		mw := RequireMembership(nil, rdb)
		var capturedClaimRole, capturedEventRole, capturedEffectiveRole string
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c, ok := auth.GetUserClaims(r.Context()); ok {
				capturedClaimRole = c.Role
			}
			if evCtx, ok := FromContext(r.Context()); ok {
				capturedEventRole = evCtx.Role
			}
			capturedEffectiveRole = auth.GetEffectiveRole(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		claims := &utils.AccessTokenClaims{
			UserID: "user-admin-1",
			Email:  "admin@cadence.local",
			Role:   "user",
		}
		ev := &Event{
			ID:   "ev-1",
			Slug: "fest-2026",
			Name: "Fest 2026",
		}

		reqCtx := auth.WithUserClaims(ctx, claims)
		reqCtx = WithEvent(reqCtx, ev)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(reqCtx)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		if capturedClaimRole != "user" {
			t.Fatalf("expected AccessTokenClaims.Role to remain 'user', got %q", capturedClaimRole)
		}
		if capturedEventRole != "admin" {
			t.Fatalf("expected EventContext.Role to be 'admin', got %q", capturedEventRole)
		}
		if capturedEffectiveRole != "admin" {
			t.Fatalf("expected auth.GetEffectiveRole to be 'admin', got %q", capturedEffectiveRole)
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

func TestCreateEventOwnerAccountCreationAndUsersManagement(t *testing.T) {
	db := newTestDBPool(t)
	rdb := newTestRedisClient(t)
	t.Cleanup(func() {
		_ = rdb.Close()
	})

	repo := NewRepository(db)
	svc := NewService(repo, rdb, nil)
	ctx := context.Background()

	nano := time.Now().UnixNano()
	creatorDevID := "00000000-0000-0000-0000-000000000001"

	t.Run("owner account created in bg with explicit name and single ownership enforced", func(t *testing.T) {
		ownerEmail := fmt.Sprintf("bg-event-owner-%d@example.com", nano)
		ownerName := "Explicit Event Organizer"
		slug := fmt.Sprintf("bg-event-%d", nano)
		evName := "Background Created Event"
		initialLogo := "https://cdn.example.com/events/initial-logo.png"

		req := CreateEventRequest{
			Slug:       slug,
			Name:       evName,
			LogoURL:    &initialLogo,
			OwnerEmail: &ownerEmail,
			OwnerName:  &ownerName,
		}

		ev, err := svc.CreateEvent(ctx, req, creatorDevID)
		if err != nil {
			t.Fatalf("failed to create event: %v", err)
		}
		if ev == nil || ev.ID == "" {
			t.Fatalf("expected valid event, got nil or empty id")
		}
		if ev.LogoURL == nil || *ev.LogoURL != initialLogo {
			t.Fatalf("expected event LogoURL %q, got %v", initialLogo, ev.LogoURL)
		}

		// Update event logo via PhotoURL alias
		updatedLogo := "https://cdn.example.com/events/updated-photo.png"
		evUpdated, err := svc.UpdateEvent(ctx, ev.ID, UpdateEventRequest{
			PhotoURL: &updatedLogo,
		}, creatorDevID)
		if err != nil || evUpdated.LogoURL == nil || *evUpdated.LogoURL != updatedLogo {
			t.Fatalf("expected updated event LogoURL %q, got %v (err: %v)", updatedLogo, evUpdated, err)
		}

		var createdUserID, createdName string
		err = db.QueryRow(ctx, "SELECT id, name FROM users WHERE email = $1", ownerEmail).
			Scan(&createdUserID, &createdName)
		if err != nil {
			t.Fatalf("expected background created user to exist in users table: %v", err)
		}
		if createdName != ownerName {
			t.Errorf("expected user name %q, got %q", ownerName, createdName)
		}

		var memberRole string
		err = db.QueryRow(ctx, "SELECT role::text FROM user_events WHERE event_id = $1 AND user_id = $2", ev.ID, createdUserID).
			Scan(&memberRole)
		if err != nil || memberRole != "owner" {
			t.Fatalf("expected user_events row with role 'owner', got %q (err: %v)", memberRole, err)
		}

		// Attempting to create another event with the same owner fails single-ownership rule
		secondReq := CreateEventRequest{
			Slug:       fmt.Sprintf("second-event-%d", nano),
			Name:       "Second Event",
			OwnerEmail: &ownerEmail,
		}
		_, err = svc.CreateEvent(ctx, secondReq, creatorDevID)
		if err != ErrUserAlreadyOwnsEvent {
			t.Fatalf("expected ErrUserAlreadyOwnsEvent, got %v", err)
		}

		// Add single user & bulk users to event
		newEmail := fmt.Sprintf("ev-participant-%d@example.com", nano)
		detail, created, err := svc.AddEventUser(ctx, ev.ID, createdUserID, "owner", AddEventUserRequest{
			Email: newEmail,
			Role:  "participant",
		})
		if err != nil || !created || detail.Role != "participant" {
			t.Fatalf("failed to add event user: created=%v detail=%+v err=%v", created, detail, err)
		}

		// Delete event (dev-only operation)
		if err := svc.DeleteEvent(ctx, ev.ID, creatorDevID); err != nil {
			t.Fatalf("failed to delete event: %v", err)
		}
		_, _ = db.Exec(ctx, "DELETE FROM users WHERE email IN ($1, $2)", ownerEmail, newEmail)
	})

	t.Run("blacklisted user is never un-blacklisted by Google OAuth or Event member onboarding, and soft-delete restoration synchronizes Redis", func(t *testing.T) {
		ownerEmail := fmt.Sprintf("inv-owner-%d@example.com", nano)
		ev, err := svc.CreateEvent(ctx, CreateEventRequest{
			Slug:       fmt.Sprintf("inv-event-%d", nano),
			Name:       "Invariant Event",
			OwnerEmail: &ownerEmail,
		}, creatorDevID)
		if err != nil {
			t.Fatalf("create event: %v", err)
		}
		var ownerID string
		if err := db.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", ownerEmail).Scan(&ownerID); err != nil {
			t.Fatalf("get owner id: %v", err)
		}
		t.Cleanup(func() {
			_, _ = db.Exec(ctx, "DELETE FROM events WHERE id = $1", ev.ID)
			_, _ = db.Exec(ctx, "DELETE FROM users WHERE id = $1", ownerID)
		})

		// 1. Create a user, blacklist them in DB + Redis
		blacklistedEmail := fmt.Sprintf("blacklisted-%d@example.com", nano)
		var blacklistedID string
		if err := db.QueryRow(ctx, `
			INSERT INTO users (email, name, role, is_active)
			VALUES ($1, 'Blacklisted User', 'user', FALSE)
			RETURNING id
		`, blacklistedEmail).Scan(&blacklistedID); err != nil {
			t.Fatalf("insert blacklisted user: %v", err)
		}
		t.Cleanup(func() {
			_, _ = db.Exec(ctx, "DELETE FROM users WHERE id = $1", blacklistedID)
		})
		_ = rdb.Set(ctx, fmt.Sprintf("blocked:user:%s", blacklistedID), "blacklisted", time.Hour).Err()

		// 2. Google OAuth UpsertGoogleUser must NOT flip is_active = TRUE for a blacklisted user!
		authRepo := auth.NewRepository(db)
		upserted, err := authRepo.UpsertGoogleUser(ctx, fmt.Sprintf("gid-%d", nano), blacklistedEmail, "Blacklisted User", nil, "user")
		if err != nil {
			t.Fatalf("UpsertGoogleUser: %v", err)
		}
		if upserted.IsActive {
			t.Fatalf("expected blacklisted user to remain is_active=false after UpsertGoogleUser, got true")
		}

		// 3. Event admin calling AddEventUser or GetOrCreateUserByEmail must NOT flip is_active = TRUE for a blacklisted user!
		_, _, _ = svc.AddEventUser(ctx, ev.ID, ownerID, "owner", AddEventUserRequest{
			Email: blacklistedEmail,
			Role:  "participant",
		})
		var isActiveAfterAdd bool
		if err := db.QueryRow(ctx, "SELECT is_active FROM users WHERE id = $1", blacklistedID).Scan(&isActiveAfterAdd); err != nil {
			t.Fatalf("check is_active: %v", err)
		}
		if isActiveAfterAdd {
			t.Fatalf("expected blacklisted user to remain is_active=false after AddEventUser, got true")
		}

		// 4. Soft-deleted user re-added via AddEventUser must have deleted_at cleared, is_active restored to TRUE,
		// AND blocked:user:{id} cleared in Redis while user:min_token_version:{id} is updated!
		softDelEmail := fmt.Sprintf("softdel-%d@example.com", nano)
		var softDelID string
		if err := db.QueryRow(ctx, `
			INSERT INTO users (email, name, role, is_active, deleted_at, token_version)
			VALUES ($1, 'Soft Deleted', 'user', FALSE, CURRENT_TIMESTAMP, 2)
			RETURNING id
		`, softDelEmail).Scan(&softDelID); err != nil {
			t.Fatalf("insert soft-deleted user: %v", err)
		}
		t.Cleanup(func() {
			_, _ = db.Exec(ctx, "DELETE FROM users WHERE id = $1", softDelID)
		})
		_ = rdb.Set(ctx, fmt.Sprintf("blocked:user:%s", softDelID), "deleted", time.Hour).Err()

		if _, _, err := svc.AddEventUser(ctx, ev.ID, ownerID, "owner", AddEventUserRequest{
			Email: softDelEmail,
			Role:  "participant",
		}); err != nil {
			t.Fatalf("AddEventUser for soft-deleted user: %v", err)
		}

		var restoredActive bool
		var restoredDeletedAt *time.Time
		if err := db.QueryRow(ctx, "SELECT is_active, deleted_at FROM users WHERE id = $1", softDelID).Scan(&restoredActive, &restoredDeletedAt); err != nil {
			t.Fatalf("query restored user: %v", err)
		}
		if !restoredActive || restoredDeletedAt != nil {
			t.Fatalf("expected soft-deleted user to be restored (is_active=true, deleted_at=nil), got is_active=%v deleted_at=%v", restoredActive, restoredDeletedAt)
		}
		if exists := rdb.Exists(ctx, fmt.Sprintf("blocked:user:%s", softDelID)).Val(); exists != 0 {
			t.Fatalf("expected Redis blocked:user:%s to be cleared when soft-deleted user is restored via AddEventUser, got exists=%d", softDelID, exists)
		}
	})
}

func TestBulkAddEventUsersWithCompetitionsAndTeams(t *testing.T) {
	db := newTestDBPool(t)
	rdb := newTestRedisClient(t)

	repo := NewRepository(db)
	svc := NewService(repo, rdb, nil)
	ctx := context.Background()

	nano := time.Now().UnixNano()
	creatorDevID := "00000000-0000-0000-0000-000000000001"
	ownerEmail := fmt.Sprintf("bulk-owner-%d@example.com", nano)
	aliceEmail := fmt.Sprintf("alice-%d@example.com", nano)
	bobEmail := fmt.Sprintf("bob-%d@example.com", nano)
	charlieEmail := fmt.Sprintf("charlie-%d@example.com", nano)

	ev, err := svc.CreateEvent(ctx, CreateEventRequest{
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
		_, _ = db.Exec(context.Background(), `DELETE FROM users WHERE email IN ($1, $2, $3, $4)`, ownerEmail, aliceEmail, bobEmail, charlieEmail)
	})

	var aiCompID, webCompID string
	if err := db.QueryRow(ctx, `INSERT INTO competitions (event_id, slug, name, min_team_size, max_team_size) VALUES ($1, 'ai-track', 'AI Track', 1, 2) RETURNING id`, ev.ID).Scan(&aiCompID); err != nil {
		t.Fatalf("create AI Track: %v", err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO competitions (event_id, slug, name, min_team_size, max_team_size) VALUES ($1, 'web-track', 'Web Track', 1, 3) RETURNING id`, ev.ID).Scan(&webCompID); err != nil {
		t.Fatalf("create Web Track: %v", err)
	}

	strPtr := func(s string) *string { return &s }

	resp, err := svc.BulkAddEventUsers(ctx, ev.ID, ownerID, "owner", BulkAddEventUsersRequest{
		Users: []AddEventUserRequest{
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
}
