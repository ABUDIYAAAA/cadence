package competitions

import (
	"bytes"
	"context"
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
