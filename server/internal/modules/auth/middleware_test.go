package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/utils"
)

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
}
