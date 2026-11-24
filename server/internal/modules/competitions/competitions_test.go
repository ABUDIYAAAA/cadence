package competitions

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
		"Hack-2026",       // uppercase
		"hack_2026",       // underscore
		"-hack",           // leading hyphen
		"hack-",           // trailing hyphen
		"hack--2026",      // consecutive hyphens
		"hack 2026",       // space
		"hack.2026",       // period
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
		err := svc.AssignOrUpdateMemberRole(context.Background(), "comp-1", "actor-admin", "admin", "target-user", "admin")
		if err != ErrPermissionDenied {
			t.Fatalf("expected ErrPermissionDenied, got %v", err)
		}
	})

	t.Run("owner cannot assign owner role via update endpoint", func(t *testing.T) {
		err := svc.AssignOrUpdateMemberRole(context.Background(), "comp-1", "actor-owner", "owner", "target-user", "owner")
		if err == nil || err.Error() != "cannot assign owner role; use transfer ownership endpoint" {
			t.Fatalf("expected cannot assign owner role error, got %v", err)
		}
	})

	t.Run("owner cannot self-demote via role update", func(t *testing.T) {
		err := svc.AssignOrUpdateMemberRole(context.Background(), "comp-1", "actor-owner", "owner", "actor-owner", "participant")
		if err != ErrCannotSelfDemote {
			t.Fatalf("expected ErrCannotSelfDemote, got %v", err)
		}
	})
}

func TestValidationDTOs(t *testing.T) {
	t.Run("valid CreateCompetitionRequest passes", func(t *testing.T) {
		req := CreateCompetitionRequest{
			Slug: "hack-2026",
			Name: "Hack 2026",
		}
		errs := utils.ValidateStruct(req)
		if len(errs) > 0 {
			t.Fatalf("unexpected validation errors: %v", errs)
		}
	})

	t.Run("invalid slug too short fails", func(t *testing.T) {
		req := CreateCompetitionRequest{
			Slug: "a",
			Name: "Hack 2026",
		}
		errs := utils.ValidateStruct(req)
		if len(errs) == 0 {
			t.Fatalf("expected validation error for short slug")
		}
	})

	t.Run("invalid TransferOwnershipRequest missing UUID fails", func(t *testing.T) {
		req := TransferOwnershipRequest{
			NewOwnerUserID: "not-a-uuid",
		}
		errs := utils.ValidateStruct(req)
		if len(errs) == 0 {
			t.Fatalf("expected validation error for invalid UUID")
		}
	})

	t.Run("valid TransferOwnershipRequest with UUID passes", func(t *testing.T) {
		req := TransferOwnershipRequest{
			NewOwnerUserID: "00000000-0000-0000-0000-000000000001",
		}
		errs := utils.ValidateStruct(req)
		if len(errs) > 0 {
			t.Fatalf("unexpected validation error: %v", errs)
		}
	})

	t.Run("valid CreateCompetitionRequest with optional owner email and id passes", func(t *testing.T) {
		email := "organizer@example.com"
		uid := "00000000-0000-0000-0000-000000000002"
		req := CreateCompetitionRequest{
			Slug:       "ai-hackathon",
			Name:       "AI Hackathon",
			OwnerEmail: &email,
			OwnerID:    &uid,
		}
		errs := utils.ValidateStruct(req)
		if len(errs) > 0 {
			t.Fatalf("unexpected validation error: %v", errs)
		}
	})

	t.Run("invalid CreateCompetitionRequest with bad owner email fails", func(t *testing.T) {
		badEmail := "not-an-email"
		req := CreateCompetitionRequest{
			Slug:       "ai-hackathon",
			Name:       "AI Hackathon",
			OwnerEmail: &badEmail,
		}
		errs := utils.ValidateStruct(req)
		if len(errs) == 0 {
			t.Fatalf("expected validation error for invalid email")
		}
	})
}

func TestCreateCompetitionDevOnlyRestriction(t *testing.T) {
	// Mount router using RegisterRoutes
	r := chi.NewRouter()
	handler := NewHandler(&Service{})

	// Dummy auth middleware that reads claims injected in request context
	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}

	RegisterRoutes(r, handler, nil, nil, authMiddleware)

	t.Run("unauthenticated user is rejected with 401 by RequireDev", func(t *testing.T) {
		body := bytes.NewBufferString(`{"slug":"hack-2026","name":"Hack 2026"}`)
		req := httptest.NewRequest(http.MethodPost, "/competitions", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("regular user with global_role=user is rejected with 403", func(t *testing.T) {
		body := bytes.NewBufferString(`{"slug":"hack-2026","name":"Hack 2026"}`)
		req := httptest.NewRequest(http.MethodPost, "/competitions", body)
		req.Header.Set("Content-Type", "application/json")

		claims := &utils.AccessTokenClaims{
			UserID:     "user-regular-1",
			Email:      "regular@cadence.local",
			GlobalRole: "user",
		}
		ctx := auth.WithUserClaims(req.Context(), claims)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req.WithContext(ctx))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for global_role=user, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["error"] != "developer access required" {
			t.Fatalf("expected 'developer access required', got %v", resp["error"])
		}
	})

	t.Run("dev user with global_role=dev passes RequireDev gate", func(t *testing.T) {
		// Send invalid JSON body so it passes RequireDev and reaches HandleCreateCompetition body decoder
		body := bytes.NewBufferString(`{invalid-json`)
		req := httptest.NewRequest(http.MethodPost, "/competitions", body)
		req.Header.Set("Content-Type", "application/json")

		claims := &utils.AccessTokenClaims{
			UserID:     "user-dev-1",
			Email:      "dev@cadence.local",
			GlobalRole: "dev",
		}
		ctx := auth.WithUserClaims(req.Context(), claims)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req.WithContext(ctx))
		// Passed RequireDev and failed on invalid request body in handler (400)
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
		_ = rdb.Set(ctx, "comp:comp-1:member:user-banned-1", "banned", time.Minute).Err()

		mw := RequireMembership(nil, rdb)
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		claims := &utils.AccessTokenClaims{
			UserID:     "user-banned-1",
			Email:      "banned@cadence.local",
			GlobalRole: "user",
		}
		comp := &Competition{
			ID:   "comp-1",
			Slug: "hack-2026",
			Name: "Hack 2026",
		}

		reqCtx := auth.WithUserClaims(ctx, claims)
		reqCtx = WithCompetition(reqCtx, comp)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(reqCtx)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for banned member, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["error"] != "you are banned from this competition" {
			t.Fatalf("expected 'you are banned from this competition', got %v", resp["error"])
		}
	})

	t.Run("Dev bypass grants owner in RequireMembership", func(t *testing.T) {
		mw := RequireMembership(nil, nil)
		var capturedRole string
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			compCtx, ok := FromContext(r.Context())
			if ok {
				capturedRole = compCtx.Role
			}
			w.WriteHeader(http.StatusOK)
		}))

		claims := &utils.AccessTokenClaims{
			UserID:     "user-dev-1",
			Email:      "dev@cadence.local",
			GlobalRole: "dev",
		}
		comp := &Competition{
			ID:   "comp-1",
			Slug: "hack-2026",
			Name: "Hack 2026",
		}

		ctx := auth.WithUserClaims(context.Background(), claims)
		ctx = WithCompetition(ctx, comp)

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
}

func TestCompetitionUserRoutesGating(t *testing.T) {
	rdb := newTestRedisClient(t)
	defer rdb.Close()
	r := chi.NewRouter()
	handler := NewHandler(&Service{})

	// Dummy auth middleware that accepts context claims
	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}

	RegisterRoutes(r, handler, nil, rdb, authMiddleware)

	setupContext := func(role, globalRole string) context.Context {
		ctx := context.Background()
		_ = rdb.Set(ctx, "comp:comp-1:member:u-test-1", role, time.Minute).Err()

		claims := &utils.AccessTokenClaims{
			UserID:     "u-test-1",
			Email:      "test@cadence.local",
			GlobalRole: globalRole,
		}
		comp := &Competition{
			ID:   "comp-1",
			Slug: "hack-2026",
			Name: "Hack 2026",
		}
		compCtx := &Context{
			CompetitionID: "comp-1",
			Slug:          "hack-2026",
			Name:          "Hack 2026",
			UserID:        "u-test-1",
			Role:          role,
		}
		reqCtx := auth.WithUserClaims(ctx, claims)
		reqCtx = WithCompetition(reqCtx, comp)
		return WithCompetitionContext(reqCtx, compCtx)
	}

	t.Run("Volunteer can view competition users", func(t *testing.T) {
		ctx := setupContext("volunteer", "user")
		req := httptest.NewRequest(http.MethodGet, "/c/hack-2026/users", nil).WithContext(ctx)
		rec := httptest.NewRecorder()

		// Handler will attempt service call and return 500 since repo is nil, which confirms it passed the role gate
		defer func() { _ = recover() }()
		r.ServeHTTP(rec, req)
		if rec.Code == http.StatusForbidden {
			t.Fatalf("volunteer should be allowed to view users, got 403 Forbidden")
		}
	})

	t.Run("Judge cannot view competition users", func(t *testing.T) {
		ctx := setupContext("judge", "user")
		req := httptest.NewRequest(http.MethodGet, "/c/hack-2026/users", nil).WithContext(ctx)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("judge must receive 403 Forbidden on /c/slug/users, got %d", rec.Code)
		}
	})

	t.Run("Volunteer cannot ban members", func(t *testing.T) {
		ctx := setupContext("volunteer", "user")
		req := httptest.NewRequest(http.MethodPost, "/c/hack-2026/members/victim-id/ban", bytes.NewBufferString("{}")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("volunteer must receive 403 Forbidden on ban, got %d", rec.Code)
		}
	})

	t.Run("Volunteer cannot add competition users", func(t *testing.T) {
		ctx := setupContext("volunteer", "user")
		req := httptest.NewRequest(http.MethodPost, "/c/hack-2026/users", bytes.NewBufferString(`{"email":"foo@example.com"}`)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("volunteer must receive 403 Forbidden on add users, got %d", rec.Code)
		}
	})

	t.Run("Volunteer cannot bulk add competition users", func(t *testing.T) {
		ctx := setupContext("volunteer", "user")
		req := httptest.NewRequest(http.MethodPost, "/c/hack-2026/users/bulk", bytes.NewBufferString(`{"users":[{"email":"foo@example.com"}]}`)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("volunteer must receive 403 Forbidden on bulk add users, got %d", rec.Code)
		}
	})

	t.Run("Judge cannot add competition users", func(t *testing.T) {
		ctx := setupContext("judge", "user")
		req := httptest.NewRequest(http.MethodPost, "/c/hack-2026/users", bytes.NewBufferString(`{"email":"foo@example.com"}`)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("judge must receive 403 Forbidden on add users, got %d", rec.Code)
		}
	})

	t.Run("Admin can access add users route", func(t *testing.T) {
		ctx := setupContext("admin", "user")
		req := httptest.NewRequest(http.MethodPost, "/c/hack-2026/users", bytes.NewBufferString(`{"email":"foo@example.com"}`)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		defer func() { _ = recover() }()
		r.ServeHTTP(rec, req)
		if rec.Code == http.StatusForbidden {
			t.Fatalf("admin should pass role gate on add users, got 403 Forbidden")
		}
	})

	t.Run("Owner can access bulk add users route", func(t *testing.T) {
		ctx := setupContext("owner", "user")
		req := httptest.NewRequest(http.MethodPost, "/c/hack-2026/users/bulk", bytes.NewBufferString(`{"users":[{"email":"foo@example.com"}]}`)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		defer func() { _ = recover() }()
		r.ServeHTTP(rec, req)
		if rec.Code == http.StatusForbidden {
			t.Fatalf("owner should pass role gate on bulk add users, got 403 Forbidden")
		}
	})

	t.Run("Admin can access ban route", func(t *testing.T) {
		ctx := setupContext("admin", "user")
		req := httptest.NewRequest(http.MethodPost, "/c/hack-2026/members/victim-id/ban", bytes.NewBufferString("{}")).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		defer func() { _ = recover() }()
		r.ServeHTTP(rec, req)
		if rec.Code == http.StatusForbidden {
			t.Fatalf("admin should pass role gate on ban, got 403 Forbidden")
		}
	})
}

func TestDeriveNameFromEmail(t *testing.T) {
	cases := []struct {
		email    string
		expected string
	}{
		{"john.doe@example.com", "John Doe"},
		{"sarah_connor@skynet.com", "Sarah Connor"},
		{"alex-turner@monkeys.co", "Alex Turner"},
		{"organizer@cadence.io", "Organizer"},
		{"", "Competition Owner"},
		{"@invalid.com", "Competition Owner"},
	}

	for _, tc := range cases {
		got := deriveNameFromEmail(tc.email)
		if got != tc.expected {
			t.Errorf("deriveNameFromEmail(%q) = %q; want %q", tc.email, got, tc.expected)
		}
	}
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
	return pool
}

func TestCreateCompetitionOwnerAccountCreationInBackground(t *testing.T) {
	db := newTestDBPool(t)
	defer db.Close()
	rdb := newTestRedisClient(t)
	defer rdb.Close()

	repo := NewRepository(db)
	svc := NewService(repo, rdb, nil)
	ctx := context.Background()

	nano := time.Now().UnixNano()
	creatorDevID := "00000000-0000-0000-0000-000000000001" // seeded admin/dev

	t.Run("owner account created in bg with explicit name", func(t *testing.T) {
		ownerEmail := fmt.Sprintf("bg-owner-%d@example.com", nano)
		ownerName := "Explicit Organizer"
		slug := fmt.Sprintf("bg-comp-%d", nano)
		compName := "Background Created Comp"

		// Confirm owner does not exist prior to competition creation
		var preCount int
		err := db.QueryRow(ctx, "SELECT count(*) FROM users WHERE email = $1", ownerEmail).Scan(&preCount)
		if err != nil || preCount != 0 {
			t.Fatalf("expected user to not exist beforehand: %v", err)
		}

		req := CreateCompetitionRequest{
			Slug:       slug,
			Name:       compName,
			OwnerEmail: &ownerEmail,
			OwnerName:  &ownerName,
		}

		comp, err := svc.CreateCompetition(ctx, req, creatorDevID)
		if err != nil {
			t.Fatalf("failed to create competition: %v", err)
		}
		if comp == nil || comp.ID == "" {
			t.Fatalf("expected valid competition, got nil or empty id")
		}

		// Verify user account was created in background in users table
		var createdUserID, createdName, globalRole string
		var isActive bool
		err = db.QueryRow(ctx, "SELECT id, name, global_role, is_active FROM users WHERE email = $1", ownerEmail).
			Scan(&createdUserID, &createdName, &globalRole, &isActive)
		if err != nil {
			t.Fatalf("expected background created user to exist in users table: %v", err)
		}
		if createdName != ownerName {
			t.Errorf("expected user name %q, got %q", ownerName, createdName)
		}
		if globalRole != "user" {
			t.Errorf("expected global_role user, got %q", globalRole)
		}
		if !isActive {
			t.Errorf("expected is_active true")
		}

		// Verify user_competitions binds the newly created user as owner
		var memberRole string
		err = db.QueryRow(ctx, "SELECT role::text FROM user_competitions WHERE competition_id = $1 AND user_id = $2", comp.ID, createdUserID).
			Scan(&memberRole)
		if err != nil {
			t.Fatalf("expected user_competitions row for created owner: %v", err)
		}
		if memberRole != "owner" {
			t.Errorf("expected role 'owner', got %q", memberRole)
		}

		// Verify Redis cache was populated
		roleCached, err := rdb.Get(ctx, fmt.Sprintf("comp:%s:member:%s", comp.ID, createdUserID)).Result()
		if err != nil || roleCached != "owner" {
			t.Errorf("expected Redis role cached as 'owner', got %v (err: %v)", roleCached, err)
		}
		isMember, err := rdb.SIsMember(ctx, fmt.Sprintf("user:%s:competitions", createdUserID), comp.ID).Result()
		if err != nil || !isMember {
			t.Errorf("expected Redis user competitions set to contain %s", comp.ID)
		}

		// Subtest: Attempting to create another competition with the same owner fails single-ownership rule
		t.Run("single ownership enforced for background-created owner", func(t *testing.T) {
			secondReq := CreateCompetitionRequest{
				Slug:       fmt.Sprintf("second-comp-%d", nano),
				Name:       "Second Competition",
				OwnerEmail: &ownerEmail,
			}
			_, err := svc.CreateCompetition(ctx, secondReq, creatorDevID)
			if err != ErrUserAlreadyOwnsCompetition {
				t.Fatalf("expected ErrUserAlreadyOwnsCompetition, got %v", err)
			}
		})
	})

	t.Run("owner account created in bg with derived name when owner_name omitted", func(t *testing.T) {
		ownerEmail := fmt.Sprintf("alex.turner.%d@example.com", nano)
		slug := fmt.Sprintf("derived-comp-%d", nano)
		compName := "Derived Name Comp"

		req := CreateCompetitionRequest{
			Slug:       slug,
			Name:       compName,
			OwnerEmail: &ownerEmail,
		}

		comp, err := svc.CreateCompetition(ctx, req, creatorDevID)
		if err != nil {
			t.Fatalf("failed to create competition: %v", err)
		}

		var createdName string
		err = db.QueryRow(ctx, "SELECT name FROM users WHERE email = $1", ownerEmail).Scan(&createdName)
		if err != nil {
			t.Fatalf("expected background user: %v", err)
		}
		expectedPrefix := "Alex Turner"
		if len(createdName) < len(expectedPrefix) || createdName[:len(expectedPrefix)] != expectedPrefix {
			t.Errorf("expected name to start with %q, got %q", expectedPrefix, createdName)
		}
		_ = comp
	})

	t.Run("transfer ownership to non-existent user creates account in bg", func(t *testing.T) {
		// Create competition with dev as creator/owner
		slug := fmt.Sprintf("transfer-comp-%d", nano)
		initialOwnerEmail := fmt.Sprintf("initial-owner-%d@example.com", nano)
		comp, err := svc.CreateCompetition(ctx, CreateCompetitionRequest{
			Slug:       slug,
			Name:       "Transfer Test Comp",
			OwnerEmail: &initialOwnerEmail,
		}, creatorDevID)
		if err != nil {
			t.Fatalf("failed to create comp: %v", err)
		}

		var initialOwnerID string
		_ = db.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", initialOwnerEmail).Scan(&initialOwnerID)

		// Transfer ownership to brand new email
		targetNewEmail := fmt.Sprintf("transferred-target-%d@example.com", nano)
		targetName := "New Transferred Owner"

		err = svc.TransferOwnership(ctx, comp.ID, initialOwnerID, TransferOwnershipRequest{
			NewOwnerEmail: &targetNewEmail,
			NewOwnerName:  &targetName,
		})
		if err != nil {
			t.Fatalf("expected successful ownership transfer: %v", err)
		}

		// Verify target user was created in users table
		var newUserID, newUserName string
		err = db.QueryRow(ctx, "SELECT id, name FROM users WHERE email = $1", targetNewEmail).Scan(&newUserID, &newUserName)
		if err != nil {
			t.Fatalf("expected new user created in bg: %v", err)
		}
		if newUserName != targetName {
			t.Errorf("expected user name %q, got %q", targetName, newUserName)
		}

		// Verify new owner has 'owner' role and old owner demoted to 'admin'
		var newRole, oldRole string
		_ = db.QueryRow(ctx, "SELECT role::text FROM user_competitions WHERE competition_id = $1 AND user_id = $2", comp.ID, newUserID).Scan(&newRole)
		_ = db.QueryRow(ctx, "SELECT role::text FROM user_competitions WHERE competition_id = $1 AND user_id = $2", comp.ID, initialOwnerID).Scan(&oldRole)

		if newRole != "owner" {
			t.Errorf("expected new role 'owner', got %q", newRole)
		}
		if oldRole != "admin" {
			t.Errorf("expected old role 'admin', got %q", oldRole)
		}
	})
}

func TestAddCompetitionUsersAndBulkDetection(t *testing.T) {
	db := newTestDBPool(t)
	defer db.Close()
	rdb := newTestRedisClient(t)
	defer rdb.Close()

	repo := NewRepository(db)
	svc := NewService(repo, rdb, nil)
	ctx := context.Background()

	nano := time.Now().UnixNano()
	ownerEmail := fmt.Sprintf("comp-owner-%d@example.com", nano)
	comp, err := svc.CreateCompetition(ctx, CreateCompetitionRequest{
		Slug:       fmt.Sprintf("bulk-add-comp-%d", nano),
		Name:       "Bulk Add Competition",
		OwnerEmail: &ownerEmail,
	}, "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("failed to create competition: %v", err)
	}

	var ownerID string
	_ = db.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", ownerEmail).Scan(&ownerID)

	// Pre-create an existing user in the database (who is not in this competition yet)
	existingUserEmail := fmt.Sprintf("pre-existing-%d@example.com", nano)
	existingUserName := "Pre Existing User"
	var existingUserID string
	err = db.QueryRow(ctx, `
		INSERT INTO users (email, name, global_role, is_active)
		VALUES ($1, $2, 'user', TRUE)
		RETURNING id
	`, existingUserEmail, existingUserName).Scan(&existingUserID)
	if err != nil {
		t.Fatalf("failed to insert pre-existing user: %v", err)
	}

	t.Run("single user add: creates account in bg when user does not exist", func(t *testing.T) {
		newEmail := fmt.Sprintf("new-single-%d@example.com", nano)
		detail, created, err := svc.AddCompetitionUser(ctx, comp.ID, ownerID, "owner", AddCompetitionUserRequest{
			Email: newEmail,
			Role:  "participant",
		})
		if err != nil {
			t.Fatalf("unexpected error adding user: %v", err)
		}
		if !created {
			t.Fatalf("expected created to be true for non-existent user")
		}
		if detail.Email != newEmail {
			t.Errorf("expected email %q, got %q", newEmail, detail.Email)
		}
		if detail.Role != "participant" {
			t.Errorf("expected role 'participant', got %q", detail.Role)
		}

		// Verify Redis
		roleCached, err := rdb.Get(ctx, fmt.Sprintf("comp:%s:member:%s", comp.ID, detail.ID)).Result()
		if err != nil || roleCached != "participant" {
			t.Errorf("expected Redis role 'participant', got %v (err: %v)", roleCached, err)
		}
		isMember, err := rdb.SIsMember(ctx, fmt.Sprintf("user:%s:competitions", detail.ID), comp.ID).Result()
		if err != nil || !isMember {
			t.Errorf("expected Redis user competitions to have %s", comp.ID)
		}
	})

	t.Run("single user add: links existing account without duplicate creation", func(t *testing.T) {
		detail, created, err := svc.AddCompetitionUser(ctx, comp.ID, ownerID, "owner", AddCompetitionUserRequest{
			Email: existingUserEmail,
			Role:  "volunteer",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created {
			t.Fatalf("expected created to be false for pre-existing user")
		}
		if detail.ID != existingUserID {
			t.Errorf("expected user ID %s, got %s", existingUserID, detail.ID)
		}
		if detail.Role != "volunteer" {
			t.Errorf("expected role 'volunteer', got %q", detail.Role)
		}
	})

	t.Run("bulk add: correctly identifies which users need accounts and which already exist", func(t *testing.T) {
		newUser1 := fmt.Sprintf("bulk-user-1-%d@example.com", nano)
		newUser2 := fmt.Sprintf("bulk-user-2-%d@example.com", nano)
		existingEmail := existingUserEmail // already created above

		req := BulkAddCompetitionUsersRequest{
			Users: []AddCompetitionUserRequest{
				{Email: newUser1, Role: "participant"},
				{Email: newUser2, Role: "judge"},
				{Email: existingEmail, Role: "judge"},        // existing user, update role
				{Email: "not-an-email", Role: "participant"}, // invalid email, should be skipped
			},
		}

		resp, err := svc.BulkAddCompetitionUsers(ctx, comp.ID, ownerID, "owner", req)
		if err != nil {
			t.Fatalf("unexpected bulk add error: %v", err)
		}

		if resp.TotalProcessed != 4 {
			t.Errorf("expected TotalProcessed 4, got %d", resp.TotalProcessed)
		}
		if resp.TotalCreated != 2 {
			t.Errorf("expected TotalCreated 2 (the 2 new users), got %d", resp.TotalCreated)
		}
		if resp.TotalAdded != 1 {
			t.Errorf("expected TotalAdded 1 (the 1 existing user), got %d", resp.TotalAdded)
		}
		if resp.SkippedCount != 1 {
			t.Errorf("expected SkippedCount 1, got %d", resp.SkippedCount)
		}
		if len(resp.Users) != 3 {
			t.Errorf("expected 3 successful users, got %d", len(resp.Users))
		}
	})

	t.Run("role hierarchy: admin cannot add another user as admin", func(t *testing.T) {
		adminEmail := fmt.Sprintf("comp-admin-%d@example.com", nano)
		adminDetail, _, err := svc.AddCompetitionUser(ctx, comp.ID, ownerID, "owner", AddCompetitionUserRequest{
			Email: adminEmail,
			Role:  "admin",
		})
		if err != nil {
			t.Fatalf("failed to add admin: %v", err)
		}

		targetEmail := fmt.Sprintf("target-by-admin-%d@example.com", nano)
		_, _, err = svc.AddCompetitionUser(ctx, comp.ID, adminDetail.ID, "admin", AddCompetitionUserRequest{
			Email: targetEmail,
			Role:  "admin", // Admin trying to assign admin role
		})
		if err != ErrPermissionDenied {
			t.Fatalf("expected ErrPermissionDenied, got %v", err)
		}
	})
}
