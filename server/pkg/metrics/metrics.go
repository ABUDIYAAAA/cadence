package metrics

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

var (
	// HTTP Metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "cadence",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests processed by Cadence API server",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "cadence",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Histogram of HTTP request processing duration in seconds",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	HTTPInFlightRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "cadence",
			Subsystem: "http",
			Name:      "in_flight_requests",
			Help:      "Current count of active in-flight HTTP requests being handled",
		},
	)

	HTTPRequestSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "cadence",
			Subsystem: "http",
			Name:      "request_size_bytes",
			Help:      "Size of incoming HTTP request payloads in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 6), // 100B, 1KB, 10KB, 100KB, 1MB, 10MB
		},
		[]string{"method", "path"},
	)

	HTTPResponseSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "cadence",
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "Size of outgoing HTTP response bodies in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 6),
		},
		[]string{"method", "path"},
	)

	// Auth & Security Metrics
	AuthEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "cadence",
			Subsystem: "auth",
			Name:      "events_total",
			Help:      "Total count of authentication lifecycle operations",
		},
		[]string{"action", "status"},
	)

	SecurityEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "cadence",
			Subsystem: "security",
			Name:      "events_total",
			Help:      "Total count of security threats detected (hijacking, replay, blocklist)",
		},
		[]string{"event_type"},
	)

	RateLimitHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "cadence",
			Subsystem: "ratelimit",
			Name:      "hits_total",
			Help:      "Total rate limiter checks evaluated",
		},
		[]string{"limiter_type", "status"},
	)

	// Domain & Hackathon Operations Metrics
	TicketsScannedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "cadence",
			Subsystem: "ticketing",
			Name:      "scanned_total",
			Help:      "Total digital event tickets scanned at check-in stations",
		},
		[]string{"status"},
	)

	JudgingScoresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "cadence",
			Subsystem: "judging",
			Name:      "scores_total",
			Help:      "Total judging rubrics and evaluations saved or submitted",
		},
		[]string{"status"},
	)

	NotificationsDispatchedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "cadence",
			Subsystem: "notifications",
			Name:      "dispatched_total",
			Help:      "Total announcements and push alerts processed",
		},
		[]string{"audience_type", "channel", "status"},
	)

	// Database Backup Metrics
	BackupRunsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "cadence",
			Subsystem: "backup",
			Name:      "runs_total",
			Help:      "Total database backup runs by outcome (success, dump_failed, upload_failed)",
		},
		[]string{"status"},
	)

	BackupLastRunTimestamp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "cadence",
			Subsystem: "backup",
			Name:      "last_run_timestamp_seconds",
			Help:      "Unix time of the most recent backup attempt, successful or not",
		},
	)

	BackupLastSuccessTimestamp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "cadence",
			Subsystem: "backup",
			Name:      "last_success_timestamp_seconds",
			Help:      "Unix time of the most recent backup that was uploaded successfully",
		},
	)

	BackupLastSizeBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "cadence",
			Subsystem: "backup",
			Name:      "last_size_bytes",
			Help:      "Size in bytes of the most recent successfully uploaded backup",
		},
	)

	BackupDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "cadence",
			Subsystem: "backup",
			Name:      "duration_seconds",
			Help:      "Time taken by each backup stage (dump, upload)",
			Buckets:   []float64{0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"stage"},
	)

	BackupEnabled = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "cadence",
			Subsystem: "backup",
			Name:      "enabled",
			Help:      "1 when scheduled backups are configured and running, 0 otherwise",
		},
	)
)

// AuthMiddleware protects the /metrics endpoint with a Bearer token.
// Uses crypto/subtle.ConstantTimeCompare to prevent timing attacks.
func AuthMiddleware(expectedToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expectedToken == "" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			token := ""
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			} else if qToken := r.URL.Query().Get("token"); qToken != "" {
				token = qToken
			}

			if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="cadence-metrics"`)
				http.Error(w, "Unauthorized: invalid or missing scrape token", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Middleware returns a Chi HTTP middleware that instruments RED metrics.
// It normalizes Chi route patterns (e.g. /api/v1/users/{id}) to prevent high metric cardinality.
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip metrics endpoint itself to prevent self-sampling loop
			if r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			HTTPInFlightRequests.Inc()
			defer HTTPInFlightRequests.Dec()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			reqSize := computeApproximateRequestSize(r)

			next.ServeHTTP(ww, r)

			duration := time.Since(start).Seconds()
			statusStr := strconv.Itoa(ww.Status())
			if ww.Status() == 0 {
				statusStr = "200"
			}

			// Extract Chi parameterized route pattern to avoid UUID cardinality explosion
			routePattern := "unmatched"
			if rctx := chi.RouteContext(r.Context()); rctx != nil {
				if pattern := rctx.RoutePattern(); pattern != "" {
					routePattern = pattern
				}
			}

			HTTPRequestsTotal.WithLabelValues(r.Method, routePattern, statusStr).Inc()
			HTTPRequestDuration.WithLabelValues(r.Method, routePattern).Observe(duration)
			HTTPRequestSizeBytes.WithLabelValues(r.Method, routePattern).Observe(float64(reqSize))
			HTTPResponseSizeBytes.WithLabelValues(r.Method, routePattern).Observe(float64(ww.BytesWritten()))
		})
	}
}

func computeApproximateRequestSize(r *http.Request) int64 {
	size := int64(0)
	if r.URL != nil {
		size += int64(len(r.URL.String()))
	}
	size += int64(len(r.Method))
	size += int64(len(r.Proto))
	for name, values := range r.Header {
		size += int64(len(name))
		for _, value := range values {
			size += int64(len(value))
		}
	}
	if r.ContentLength > 0 {
		size += r.ContentLength
	}
	return size
}

// DBStatsCollector periodically gathers pgxpool connection metrics.
type DBStatsCollector struct {
	pool            *pgxpool.Pool
	connectionsDesc *prometheus.Desc
	waitsDesc       *prometheus.Desc
	cancelsDesc     *prometheus.Desc
	acquiresDesc    *prometheus.Desc
}

func NewDBStatsCollector(pool *pgxpool.Pool) *DBStatsCollector {
	return &DBStatsCollector{
		pool: pool,
		connectionsDesc: prometheus.NewDesc(
			"cadence_db_pool_connections",
			"Current count of PostgreSQL connections in the pgxpool",
			[]string{"state"},
			nil,
		),
		waitsDesc: prometheus.NewDesc(
			"cadence_db_pool_acquire_waits_total",
			"Total number of times a connection acquire had to wait because no idle connection was available",
			nil,
			nil,
		),
		cancelsDesc: prometheus.NewDesc(
			"cadence_db_pool_canceled_acquires_total",
			"Total number of times a connection acquire was canceled",
			nil,
			nil,
		),
		acquiresDesc: prometheus.NewDesc(
			"cadence_db_pool_acquires_total",
			"Total number of successful connection acquires",
			nil,
			nil,
		),
	}
}

func (c *DBStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.connectionsDesc
	ch <- c.waitsDesc
	ch <- c.cancelsDesc
	ch <- c.acquiresDesc
}

func (c *DBStatsCollector) Collect(ch chan<- prometheus.Metric) {
	if c.pool == nil {
		return
	}
	stat := c.pool.Stat()

	ch <- prometheus.MustNewConstMetric(
		c.connectionsDesc,
		prometheus.GaugeValue,
		float64(stat.TotalConns()),
		"total",
	)
	ch <- prometheus.MustNewConstMetric(
		c.connectionsDesc,
		prometheus.GaugeValue,
		float64(stat.AcquiredConns()),
		"acquired",
	)
	ch <- prometheus.MustNewConstMetric(
		c.connectionsDesc,
		prometheus.GaugeValue,
		float64(stat.IdleConns()),
		"idle",
	)
	ch <- prometheus.MustNewConstMetric(
		c.connectionsDesc,
		prometheus.GaugeValue,
		float64(stat.MaxConns()),
		"max",
	)
	ch <- prometheus.MustNewConstMetric(
		c.waitsDesc,
		prometheus.CounterValue,
		float64(stat.EmptyAcquireCount()),
	)
	ch <- prometheus.MustNewConstMetric(
		c.cancelsDesc,
		prometheus.CounterValue,
		float64(stat.CanceledAcquireCount()),
	)
	ch <- prometheus.MustNewConstMetric(
		c.acquiresDesc,
		prometheus.CounterValue,
		float64(stat.AcquireCount()),
	)
}

// RegisterDBStats registers PostgreSQL connection pool metrics with Prometheus.
func RegisterDBStats(pool *pgxpool.Pool) {
	if pool != nil {
		collector := NewDBStatsCollector(pool)
		if err := prometheus.Register(collector); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				panic(err)
			}
		}
	}
}

// RedisStatsCollector periodically gathers Redis connection pool metrics.
type RedisStatsCollector struct {
	client       *redis.Client
	connsDesc    *prometheus.Desc
	hitsDesc     *prometheus.Desc
	missesDesc   *prometheus.Desc
	timeoutsDesc *prometheus.Desc
}

func NewRedisStatsCollector(client *redis.Client) *RedisStatsCollector {
	return &RedisStatsCollector{
		client: client,
		connsDesc: prometheus.NewDesc(
			"cadence_redis_pool_connections",
			"Current count of Redis connections in the pool",
			[]string{"state"},
			nil,
		),
		hitsDesc: prometheus.NewDesc(
			"cadence_redis_pool_hits_total",
			"Total number of times a free connection was found in the Redis pool",
			nil,
			nil,
		),
		missesDesc: prometheus.NewDesc(
			"cadence_redis_pool_misses_total",
			"Total number of times a free connection was not found in the Redis pool",
			nil,
			nil,
		),
		timeoutsDesc: prometheus.NewDesc(
			"cadence_redis_pool_timeouts_total",
			"Total number of times a Redis connection wait timeout occurred",
			nil,
			nil,
		),
	}
}

func (c *RedisStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.connsDesc
	ch <- c.hitsDesc
	ch <- c.missesDesc
	ch <- c.timeoutsDesc
}

func (c *RedisStatsCollector) Collect(ch chan<- prometheus.Metric) {
	if c.client == nil {
		return
	}
	stats := c.client.PoolStats()
	if stats == nil {
		return
	}

	ch <- prometheus.MustNewConstMetric(
		c.connsDesc,
		prometheus.GaugeValue,
		float64(stats.TotalConns),
		"total",
	)
	ch <- prometheus.MustNewConstMetric(
		c.connsDesc,
		prometheus.GaugeValue,
		float64(stats.IdleConns),
		"idle",
	)
	ch <- prometheus.MustNewConstMetric(
		c.connsDesc,
		prometheus.GaugeValue,
		float64(stats.StaleConns),
		"stale",
	)
	ch <- prometheus.MustNewConstMetric(
		c.hitsDesc,
		prometheus.CounterValue,
		float64(stats.Hits),
	)
	ch <- prometheus.MustNewConstMetric(
		c.missesDesc,
		prometheus.CounterValue,
		float64(stats.Misses),
	)
	ch <- prometheus.MustNewConstMetric(
		c.timeoutsDesc,
		prometheus.CounterValue,
		float64(stats.Timeouts),
	)
}

// RegisterRedisStats registers Redis connection pool metrics with Prometheus.
func RegisterRedisStats(client *redis.Client) {
	if client != nil {
		collector := NewRedisStatsCollector(client)
		if err := prometheus.Register(collector); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				panic(err)
			}
		}
	}
}

// InstrumentAuthEvent records authentication outcomes.
func InstrumentAuthEvent(action string, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	AuthEventsTotal.WithLabelValues(action, status).Inc()
}

// InstrumentSecurityThreat records suspicious security and replay activity.
func InstrumentSecurityThreat(eventType string) {
	SecurityEventsTotal.WithLabelValues(eventType).Inc()
}

// InstrumentRateLimit records rate limit checks.
func InstrumentRateLimit(limiterType string, allowed bool) {
	status := "allowed"
	if !allowed {
		status = "blocked"
	}
	RateLimitHitsTotal.WithLabelValues(limiterType, status).Inc()
}

// InstrumentTicketScan records ticket QR scan verification.
func InstrumentTicketScan(status string) {
	TicketsScannedTotal.WithLabelValues(status).Inc()
}

// InstrumentScoreSubmission records judging rubric save/submit.
func InstrumentScoreSubmission(status string) {
	JudgingScoresTotal.WithLabelValues(status).Inc()
}

// InstrumentNotification records notification dispatches.
func InstrumentNotification(audienceType, channel, status string) {
	NotificationsDispatchedTotal.WithLabelValues(audienceType, channel, status).Inc()
}
