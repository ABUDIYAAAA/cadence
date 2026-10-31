package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/metrics"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"os"
)

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	_ = godotenv.Load("../../.env")
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://127.0.0.1:6379/15"
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Skipf("skipping rate limit Redis test: invalid REDIS_URL: %v", err)
	}
	// Always use isolated DB 15 for unit tests
	opts.DB = 15

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping rate limit Redis test: local Redis not reachable: %v", err)
	}
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush test redis db: %v", err)
	}
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background())
		_ = client.Close()
	})
	return client
}

func TestAtomicSlidingWindowLimiter(t *testing.T) {
	rdb := newTestRedis(t)
	limiter := NewLimiter(rdb)
	ctx := context.Background()

	const limit = 25
	const totalRequests = 60
	var allowedCount int32
	var blockedCount int32

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			res, err := limiter.Allow(ctx, "ratelimit:test:atomic", limit, time.Minute)
			if err != nil {
				t.Errorf("Allow returned error: %v", err)
				return
			}
			if res.Allowed {
				atomic.AddInt32(&allowedCount, 1)
			} else {
				atomic.AddInt32(&blockedCount, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if allowedCount != limit {
		t.Fatalf("expected exactly %d allowed requests under concurrent load, got %d (blocked=%d)", limit, allowedCount, blockedCount)
	}
	if blockedCount != totalRequests-limit {
		t.Fatalf("expected %d blocked requests, got %d", totalRequests-limit, blockedCount)
	}
}

func TestRateLimitMiddleware_CampusNATAndBuckets(t *testing.T) {
	rdb := newTestRedis(t)
	limiter := NewLimiter(rdb)

	cfg := &config.Config{
		FrontendURL:        "http://localhost:3000",
		JWTAccessSecret:    "test-jwt-access-secret-32-bytes-minimum",
		MetricsScrapeToken: "test-metrics-scrape-token",
	}
	cookieManager := utils.NewCookieManager("test-cookie-secret-32-bytes-minimum", "", false, "lax")

	r := chi.NewRouter()
	r.Use(metrics.Middleware())
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(DeviceIDMiddleware(cookieManager))
	r.Use(RateLimitMiddleware(limiter, cfg, cookieManager))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.With(metrics.AuthMiddleware(cfg.MetricsScrapeToken)).Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/api/v1/judging/teams/my-status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	const campusIP = "203.0.113.10:54321"

	// Pre-saturate the shared unauthenticated Campus IP bucket (ratelimit:ip:203.0.113.10)
	ctx := context.Background()
	ipKey := "ratelimit:ip:203.0.113.10"
	nowMicro := time.Now().UnixMicro()
	pipe := rdb.Pipeline()
	for i := int64(0); i < DefaultCampusIPLimit; i++ {
		pipe.ZAdd(ctx, ipKey, redis.Z{
			Score:  float64(nowMicro),
			Member: fmt.Sprintf("flood-%d", i),
		})
	}
	pipe.Expire(ctx, ipKey, time.Minute)
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatalf("seed campus ip bucket: %v", err)
	}

	t.Run("unauthenticated requests from saturated campus IP are blocked with 429", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = campusIP
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 for unauthenticated request on saturated campus IP, got %d", rec.Code)
		}
	})

	t.Run("forged or invalid JWT does not get a per-user bucket and is blocked by campus IP ceiling", func(t *testing.T) {
		forgedToken, err := utils.GenerateAccessToken("user-1", "u1@test.com", "participant", 1, "wrong-secret", 15*time.Minute)
		if err != nil {
			t.Fatalf("generate forged token: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/judging/teams/my-status", nil)
		req.RemoteAddr = campusIP
		req.Header.Set("Authorization", "Bearer "+forgedToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 for forged JWT on saturated campus IP, got %d", rec.Code)
		}
	})

	t.Run("authenticated users with verified JWT succeed even when campus IP ceiling is saturated", func(t *testing.T) {
		validToken, err := utils.GenerateAccessToken("user-verified-1", "u1@test.com", "participant", 1, cfg.JWTAccessSecret, 15*time.Minute)
		if err != nil {
			t.Fatalf("generate valid token: %v", err)
		}

		cookieRec := httptest.NewRecorder()
		cookieManager.SetSignedCookie(cookieRec, "access_token", validToken, 15*time.Minute)
		cookies := cookieRec.Result().Cookies()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/judging/teams/my-status", nil)
		req.RemoteAddr = campusIP
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for verified authenticated user on saturated campus IP, got %d", rec.Code)
		}
	})

	t.Run("blocklisted user JWT does not bypass campus IP ceiling", func(t *testing.T) {
		blockedUserID := "user-blocked-99"
		if err := rdb.Set(ctx, "blocked:user:"+blockedUserID, "1", time.Minute).Err(); err != nil {
			t.Fatalf("set blocklist key: %v", err)
		}
		validToken, err := utils.GenerateAccessToken(blockedUserID, "blocked@test.com", "participant", 1, cfg.JWTAccessSecret, 15*time.Minute)
		if err != nil {
			t.Fatalf("generate token: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/judging/teams/my-status", nil)
		req.RemoteAddr = campusIP
		req.Header.Set("Authorization", "Bearer "+validToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 for blocklisted user on saturated campus IP, got %d", rec.Code)
		}
	})

	t.Run("refresh endpoint enforces both refresh-token and stable refresh-session buckets and bypasses campus IP ceiling", func(t *testing.T) {
		cookieRec := httptest.NewRecorder()
		cookieManager.SetSignedCookie(cookieRec, DeviceIDCookieName, "stable-device-session-1", 24*time.Hour)
		cookieManager.SetSignedCookie(cookieRec, "refresh_token", "refresh-token-v1", 24*time.Hour)
		cookies := cookieRec.Result().Cookies()

		// 1. Succeeds despite saturated campus IP
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		req.RemoteAddr = campusIP
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK on /api/v1/auth/refresh with valid signed cookies, got %d", rec.Code)
		}

		// 2. Rotating the refresh token on every request still hits the stable refresh-session bucket (DefaultDeviceLimit = 60)
		for i := int64(1); i < DefaultDeviceLimit; i++ {
			rotRec := httptest.NewRecorder()
			cookieManager.SetSignedCookie(rotRec, DeviceIDCookieName, "stable-device-session-1", 24*time.Hour)
			cookieManager.SetSignedCookie(rotRec, "refresh_token", fmt.Sprintf("refresh-token-v%d", i+1), 24*time.Hour)

			rotReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
			rotReq.RemoteAddr = campusIP
			for _, c := range rotRec.Result().Cookies() {
				rotReq.AddCookie(c)
			}
			respRec := httptest.NewRecorder()
			r.ServeHTTP(respRec, rotReq)
			if respRec.Code != http.StatusOK {
				t.Fatalf("expected 200 OK on refresh #%d, got %d", i+1, respRec.Code)
			}
		}

		// 61st refresh on the same device session — even with a brand new refresh_token — must be 429'd by the stable refresh_session bucket
		overRec := httptest.NewRecorder()
		cookieManager.SetSignedCookie(overRec, DeviceIDCookieName, "stable-device-session-1", 24*time.Hour)
		cookieManager.SetSignedCookie(overRec, "refresh_token", "refresh-token-v999", 24*time.Hour)
		overReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		overReq.RemoteAddr = campusIP
		for _, c := range overRec.Result().Cookies() {
			overReq.AddCookie(c)
		}
		overResp := httptest.NewRecorder()
		r.ServeHTTP(overResp, overReq)
		if overResp.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 when stable refresh_session bucket is exceeded, got %d", overResp.Code)
		}
	})

	t.Run("CORS preflights terminate before rate limiting and do not mint _did cookies", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/judging/teams/my-status", nil)
		req.RemoteAddr = campusIP
		req.Header.Set("Origin", cfg.FrontendURL)
		req.Header.Set("Access-Control-Request-Method", "GET")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("expected OPTIONS preflight to terminate before rate limiting, got 429")
		}
		for _, c := range rec.Result().Cookies() {
			if c.Name == DeviceIDCookieName {
				t.Fatalf("expected OPTIONS preflight not to mint a %s cookie", DeviceIDCookieName)
			}
		}
	})

	t.Run("/metrics stays private (401 without token) while valid token scrapes are rate-limit-exempt", func(t *testing.T) {
		// Unauthenticated request to /metrics fails closed with 401/429 (never 200)
		unauthReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		unauthReq.RemoteAddr = "198.51.100.77:12345"
		unauthRec := httptest.NewRecorder()
		r.ServeHTTP(unauthRec, unauthReq)
		if unauthRec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized on /metrics without token, got %d", unauthRec.Code)
		}

		// Authenticated scrape succeeds even from saturated campus IP
		authReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		authReq.RemoteAddr = campusIP
		authReq.Header.Set("Authorization", "Bearer "+cfg.MetricsScrapeToken)
		authRec := httptest.NewRecorder()
		r.ServeHTTP(authRec, authReq)
		if authRec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK on /metrics with valid token from saturated campus IP, got %d", authRec.Code)
		}
	})

	t.Run("cookieless unauthenticated requests (dropping _did cookie) are throttled at DefaultDeviceLimit via IP bucket", func(t *testing.T) {
		const cookielessPeer = "198.51.100.88:44321"
		for i := int64(1); i <= DefaultDeviceLimit; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			req.RemoteAddr = cookielessPeer
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 OK on cookieless request #%d, got %d", i, rec.Code)
			}
		}

		// 61st cookieless request from the same IP must be 429'd even though no _did cookie was sent
		overReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		overReq.RemoteAddr = cookielessPeer
		overRec := httptest.NewRecorder()
		r.ServeHTTP(overRec, overReq)
		if overRec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 on cookieless request #%d, got %d", DefaultDeviceLimit+1, overRec.Code)
		}
	})

	t.Run("Bearer-only authenticated client without cookies is tracked in per-user bucket and throttled at DefaultUserLimit", func(t *testing.T) {
		bearerToken, err := utils.GenerateAccessToken("user-bearer-only", "bearer@test.com", "participant", 1, cfg.JWTAccessSecret, 15*time.Minute)
		if err != nil {
			t.Fatalf("generate bearer token: %v", err)
		}

		for i := int64(1); i <= DefaultUserLimit; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/judging/teams/my-status", nil)
			req.RemoteAddr = campusIP
			req.Header.Set("Authorization", "Bearer "+bearerToken)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 OK on Bearer request #%d, got %d", i, rec.Code)
			}
		}

		overReq := httptest.NewRequest(http.MethodGet, "/api/v1/judging/teams/my-status", nil)
		overReq.RemoteAddr = campusIP
		overReq.Header.Set("Authorization", "Bearer "+bearerToken)
		overRec := httptest.NewRecorder()
		r.ServeHTTP(overRec, overReq)
		if overRec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 on Bearer request #%d, got %d", DefaultUserLimit+1, overRec.Code)
		}
	})
}

func TestGlobalRateLimitHeaderAndConfig(t *testing.T) {
	rdb := newTestRedis(t)
	limiter := NewLimiter(rdb)

	cfg := &config.Config{
		JWTAccessSecret: "test-access-secret-32-bytes-long!",
		GlobalRateLimit: 500,
		UnauthRateLimit: 1000,
	}
	cookieManager := utils.NewCookieManager("test-cookie-secret-32-bytes-long", "", false, "lax")

	r := chi.NewRouter()
	r.Use(DeviceIDMiddleware(cookieManager))
	r.Use(RateLimitMiddleware(limiter, cfg, cookieManager))
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if limitHeader := rec.Header().Get("X-RateLimit-Limit"); limitHeader != "1000" {
		t.Fatalf("expected X-RateLimit-Limit 1000, got %s", limitHeader)
	}
	if remainingHeader := rec.Header().Get("X-RateLimit-Remaining"); remainingHeader != "999" {
		t.Fatalf("expected X-RateLimit-Remaining 999, got %s", remainingHeader)
	}
}
