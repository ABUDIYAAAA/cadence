package bootstrap_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"devclubxnst.com/cadence/internal/bootstrap"
	"devclubxnst.com/cadence/internal/config"
)

func TestHealthHandler_DegradedWhenDependenciesUnavailable(t *testing.T) {
	cfg := &config.Config{Env: "test"}

	t.Run("returns degraded status without crashing when all dependencies are nil", func(t *testing.T) {
		handler := bootstrap.HealthHandler(cfg, nil, nil, nil, nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		// Execute handler - MUST NOT PANIC or crash
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200 OK, got %d", rec.Code)
		}

		var resp map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse JSON response: %v", err)
		}

		if resp["status"] != "degraded" {
			t.Errorf("expected status 'degraded', got %q", resp["status"])
		}
		if resp["postgres"] != "down" {
			t.Errorf("expected postgres 'down', got %q", resp["postgres"])
		}
		if resp["redis"] != "down" {
			t.Errorf("expected redis 'down', got %q", resp["redis"])
		}
		if resp["rabbitmq"] != "down" {
			t.Errorf("expected rabbitmq 'down', got %q", resp["rabbitmq"])
		}
		if resp["env"] != "test" {
			t.Errorf("expected env 'test', got %q", resp["env"])
		}
	})
}
