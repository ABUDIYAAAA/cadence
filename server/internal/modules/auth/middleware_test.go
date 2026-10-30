package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://127.0.0.1:6379/15"
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Skipf("skipping Redis test: invalid REDIS_URL: %v", err)
	}
	opts.DB = 15

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping Redis test: local Redis not reachable: %v", err)
	}
	_ = client.FlushDB(ctx)
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background())
		_ = client.Close()
	})
	return client
}

func TestAuthMiddlewareAndRequireRoles(t *testing.T) {
	cfg := &config.Config{
		JWTAccessSecret: "unit-test-jwt-secret-1234567890",
		CookieSecret:    "unit-test-cookie-secret-1234567890",
		CookieSameSite:  "lax",
		JWTAccessExpiry: 15 * time.Minute,
	}
	cm := utils.NewCookieManager(cfg.CookieSecret, "", false, cfg.CookieSameSite)

	authMW := AuthMiddleware(cfg, cm, nil)
	adminOrJudgeOnly := RequireRoles("admin", "judge")

	protectedHandler := authMW(adminOrJudgeOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetUserClaims(r.Context())
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(claims.Role + ":" + claims.UserID))
	})))

	t.Run("missing authentication returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		protectedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("valid signed access_token cookie with allowed role returns 200", func(t *testing.T) {
		tok, _ := utils.GenerateAccessToken("u-judge-1", "judge@cadence.local", "judge", 1, cfg.JWTAccessSecret, 15*time.Minute)
		cookieRec := httptest.NewRecorder()
		cm.SetSignedCookie(cookieRec, "access_token", tok, 15*time.Minute)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		for _, c := range cookieRec.Result().Cookies() {
			req.AddCookie(c)
		}

		rec := httptest.NewRecorder()
		protectedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != "judge:u-judge-1" {
			t.Fatalf("expected 200 'judge:u-judge-1', got %d (%s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("valid Authorization Bearer header fallback with disallowed role returns 403", func(t *testing.T) {
		tok, _ := utils.GenerateAccessToken("u-part-1", "part@cadence.local", "participant", 1, cfg.JWTAccessSecret, 15*time.Minute)
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tok)

		rec := httptest.NewRecorder()
		protectedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for participant role, got %d", rec.Code)
		}
	})

	t.Run("stale token_version after role change is revoked immediately with 401", func(t *testing.T) {
		rdb := newTestRedis(t)
		authMWWithRedis := AuthMiddleware(cfg, cm, rdb)
		handlerWithRedis := authMWWithRedis(adminOrJudgeOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})))

		userID := "user-demoted-1"
		// Token has version 1 with judge role
		staleTok, _ := utils.GenerateAccessToken(userID, "judge@cadence.local", "judge", 1, cfg.JWTAccessSecret, 15*time.Minute)

		// Admin demoted role, bumping minimum required version to 2
		ctx := context.Background()
		if err := rdb.Set(ctx, "user:min_token_version:"+userID, "2", time.Hour).Err(); err != nil {
			t.Fatalf("set min_token_version: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+staleTok)
		rec := httptest.NewRecorder()
		handlerWithRedis.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized for stale token_version after role change, got %d", rec.Code)
		}
	})

	t.Run("deactivated user in Redis blocklist is revoked immediately with 401", func(t *testing.T) {
		rdb := newTestRedis(t)
		authMWWithRedis := AuthMiddleware(cfg, cm, rdb)
		handlerWithRedis := authMWWithRedis(adminOrJudgeOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})))

		userID := "user-deactivated-1"
		tok, _ := utils.GenerateAccessToken(userID, "judge@cadence.local", "judge", 1, cfg.JWTAccessSecret, 15*time.Minute)

		ctx := context.Background()
		if err := rdb.Set(ctx, "blocked:user:"+userID, "deactivated", time.Hour).Err(); err != nil {
			t.Fatalf("set blocked:user: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		handlerWithRedis.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized for deactivated user in blocklist, got %d", rec.Code)
		}
	})
}
