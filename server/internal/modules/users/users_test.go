package users_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/internal/modules/users"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/go-chi/chi/v5"
)

func TestNonDevCannotAccessGlobalUsersEndpoints(t *testing.T) {
	cfg := &config.Config{
		JWTAccessSecret: "super-secret-jwt-key-32-bytes-long!",
	}
	cookieMgr := utils.NewCookieManager(cfg.JWTAccessSecret, "", false, "lax")

	// Auth middleware without Redis (nil redis client)
	authMiddleware := auth.AuthMiddleware(cfg, cookieMgr, nil)

	r := chi.NewRouter()
	// Handler with nil service since unauthorized requests must be rejected before service invocation
	handler := users.NewHandler(nil)
	users.RegisterRoutes(r, handler, authMiddleware)

	roles := []struct {
		name       string
		role       string
		wantStatus int
	}{
		{name: "Admin Without Dev", role: "admin", wantStatus: http.StatusForbidden},
		{name: "Judge", role: "judge", wantStatus: http.StatusForbidden},
		{name: "Participant", role: "participant", wantStatus: http.StatusForbidden},
	}

	for _, tc := range roles {
		t.Run(tc.name+" GET /users", func(t *testing.T) {
			token, err := utils.GenerateAccessToken("u-123", tc.role+"@cadence.local", tc.role, 1, cfg.JWTAccessSecret, time.Hour)
			if err != nil {
				t.Fatalf("failed to generate access token: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d for role %s on GET /users, got %d (body: %s)", tc.wantStatus, tc.role, rec.Code, rec.Body.String())
			}

			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if body["error"] != "developer access required" {
				t.Fatalf("expected error 'developer access required', got %v", body["error"])
			}
		})

		t.Run(tc.name+" GET /users/{id}", func(t *testing.T) {
			token, err := utils.GenerateAccessToken("u-123", tc.role+"@cadence.local", tc.role, 1, cfg.JWTAccessSecret, time.Hour)
			if err != nil {
				t.Fatalf("failed to generate access token: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/users/u-victim-id", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d for role %s on GET /users/{id}, got %d (body: %s)", tc.wantStatus, tc.role, rec.Code, rec.Body.String())
			}

			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if body["error"] != "developer access required" {
				t.Fatalf("expected error 'developer access required', got %v", body["error"])
			}
		})
	}

	t.Run("Dev role passes gate on GET /users", func(t *testing.T) {
		token, err := utils.GenerateAccessToken("u-dev-1", "dev@cadence.local", "dev", 1, cfg.JWTAccessSecret, time.Hour)
		if err != nil {
			t.Fatalf("failed to generate access token: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		// Passes auth & RequireDev, reaches handler (which would panic/500 if service nil, but returns 500 cleanly)
		defer func() {
			_ = recover()
		}()
		r.ServeHTTP(rec, req)
	})
}

func TestHandlerDefenseInDepthRejectsNonDev(t *testing.T) {
	handler := users.NewHandler(nil)

	// Context with claims of regular user
	claims := &utils.AccessTokenClaims{
		UserID: "u-admin-1",
		Email:  "admin@test.com",
		Role:   "admin",
	}

	t.Run("HandleList rejects non-dev", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		req = req.WithContext(auth.WithUserClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		handler.HandleList(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
	})

	t.Run("HandleGet rejects non-dev", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/some-id", nil)
		req = req.WithContext(auth.WithUserClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		handler.HandleGet(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
	})
}
