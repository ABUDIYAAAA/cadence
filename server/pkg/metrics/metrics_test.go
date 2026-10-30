package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func readCounterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m dto.Metric
	if err := c.Write(&m); err != nil {
		t.Fatalf("write counter metric: %v", err)
	}
	if m.Counter == nil || m.Counter.Value == nil {
		return 0
	}
	return *m.Counter.Value
}

func TestMetricsAuthMiddleware(t *testing.T) {
	const expectedToken = "secret-scrape-token-123"
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("metrics_ok"))
	})

	protected := AuthMiddleware(expectedToken)(nextHandler)

	t.Run("fails closed when expected token is empty", func(t *testing.T) {
		emptyProtected := AuthMiddleware("")(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		emptyProtected.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized when expectedToken is empty, got %d", rec.Code)
		}
	})

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

	// Simulate a pre-route middleware that terminates OPTIONS preflights and 429 rate-limited requests
	// before Chi's routeHTTP executes.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if r.Header.Get("X-Simulate-429") == "1" {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/users", func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("X-Simulate-401") == "1" {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					next.ServeHTTP(w, r)
				})
			})
			r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"123"}`))
			})
		})
	})
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	const pattern = "/api/v1/users/{id}"

	// 1. Normal 200 OK request
	before200 := readCounterValue(t, HTTPRequestsTotal.WithLabelValues(http.MethodGet, pattern, "200"))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/550e8400-e29b-41d4-a716-446655440000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	after200 := readCounterValue(t, HTTPRequestsTotal.WithLabelValues(http.MethodGet, pattern, "200"))
	if after200-before200 != 1 {
		t.Fatalf("expected 200 OK metric for %s to increment by 1, got delta %v", pattern, after200-before200)
	}

	// 2. Pre-route 429 rate-limited request must resolve to normalized pattern instead of "unmatched"
	before429 := readCounterValue(t, HTTPRequestsTotal.WithLabelValues(http.MethodGet, pattern, "429"))
	req429 := httptest.NewRequest(http.MethodGet, "/api/v1/users/550e8400-e29b-41d4-a716-446655440000", nil)
	req429.Header.Set("X-Simulate-429", "1")
	rec429 := httptest.NewRecorder()
	r.ServeHTTP(rec429, req429)
	if rec429.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec429.Code)
	}
	after429 := readCounterValue(t, HTTPRequestsTotal.WithLabelValues(http.MethodGet, pattern, "429"))
	if after429-before429 != 1 {
		t.Fatalf("expected 429 metric for %s to increment by 1, got delta %v", pattern, after429-before429)
	}

	// 3. Pre-route OPTIONS preflight must resolve to normalized pattern instead of "unmatched"
	beforeOpts := readCounterValue(t, HTTPRequestsTotal.WithLabelValues(http.MethodOptions, pattern, "204"))
	reqOpts := httptest.NewRequest(http.MethodOptions, "/api/v1/users/550e8400-e29b-41d4-a716-446655440000", nil)
	reqOpts.Header.Set("Access-Control-Request-Method", "GET")
	recOpts := httptest.NewRecorder()
	r.ServeHTTP(recOpts, reqOpts)
	if recOpts.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recOpts.Code)
	}
	afterOpts := readCounterValue(t, HTTPRequestsTotal.WithLabelValues(http.MethodOptions, pattern, "204"))
	if afterOpts-beforeOpts != 1 {
		t.Fatalf("expected OPTIONS metric for %s to increment by 1, got delta %v", pattern, afterOpts-beforeOpts)
	}

	// 4. Subrouter 401 must resolve to full leaf pattern instead of "/api/v1/users/*"
	before401 := readCounterValue(t, HTTPRequestsTotal.WithLabelValues(http.MethodGet, pattern, "401"))
	req401 := httptest.NewRequest(http.MethodGet, "/api/v1/users/550e8400-e29b-41d4-a716-446655440000", nil)
	req401.Header.Set("X-Simulate-401", "1")
	rec401 := httptest.NewRecorder()
	r.ServeHTTP(rec401, req401)
	if rec401.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec401.Code)
	}
	after401 := readCounterValue(t, HTTPRequestsTotal.WithLabelValues(http.MethodGet, pattern, "401"))
	if after401-before401 != 1 {
		t.Fatalf("expected 401 metric for %s to increment by 1, got delta %v", pattern, after401-before401)
	}

	// 5. Unknown path must still resolve to "unmatched"
	beforeUnmatched := readCounterValue(t, HTTPRequestsTotal.WithLabelValues(http.MethodGet, "unmatched", "404"))
	req404 := httptest.NewRequest(http.MethodGet, "/unknown/scanner/path", nil)
	rec404 := httptest.NewRecorder()
	r.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec404.Code)
	}
	afterUnmatched := readCounterValue(t, HTTPRequestsTotal.WithLabelValues(http.MethodGet, "unmatched", "404"))
	if afterUnmatched-beforeUnmatched != 1 {
		t.Fatalf("expected 404 unmatched metric to increment by 1, got delta %v", afterUnmatched-beforeUnmatched)
	}

	// 6. /metrics self-scrape is skipped
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
