package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsAuthMiddleware(t *testing.T) {
	const expectedToken = "secret-scrape-token-123"
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("metrics_ok"))
	})

	protected := AuthMiddleware(expectedToken)(nextHandler)

	t.Run("rejects missing token with 401 and WWW-Authenticate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
		if rec.Header().Get("WWW-Authenticate") == "" {
			t.Fatal("expected WWW-Authenticate header on 401 response")
		}
	})

	t.Run("rejects invalid token with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		rec := httptest.NewRecorder()
		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("allows valid Bearer token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+expectedToken)
		rec := httptest.NewRecorder()
		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK || rec.Body.String() != "metrics_ok" {
			t.Fatalf("expected 200 OK with 'metrics_ok', got %d (%s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("allows valid query parameter token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics?token="+expectedToken, nil)
		rec := httptest.NewRecorder()
		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
	})
}

func TestREDMetricsMiddleware(t *testing.T) {
	r := chi.NewRouter()
	r.Use(Middleware())
	r.Get("/api/v1/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"123"}`))
	})
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/550e8400-e29b-41d4-a716-446655440000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	r.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /metrics, got %d", metricsRec.Code)
	}
}

func TestStatsCollectorsNilHandling(t *testing.T) {
	// Must not panic on nil clients or duplicate registration
	RegisterDBStats(nil)
	RegisterRedisStats(nil)

	dbCollector := NewDBStatsCollector(nil)
	ch := make(chan prometheus.Metric, 10)
	dbCollector.Collect(ch)
	close(ch)
	if len(ch) != 0 {
		t.Fatalf("expected 0 metrics for nil db pool, got %d", len(ch))
	}

	redisCollector := NewRedisStatsCollector(nil)
	rCh := make(chan prometheus.Metric, 10)
	redisCollector.Collect(rCh)
	close(rCh)
	if len(rCh) != 0 {
		t.Fatalf("expected 0 metrics for nil redis client, got %d", len(rCh))
	}
}
