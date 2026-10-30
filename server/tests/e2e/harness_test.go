package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/bootstrap"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcrabbitmq "github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"
)

const (
	// Pinned container images for deterministic E2E runs
	postgresImage = "postgres:16.2"
	redisImage    = "redis:7.2.4"
	rabbitmqImage = "rabbitmq:4.3.6-management"
	mailpitImage  = "axllent/mailpit:v1.21.8"

	// Deterministic test credentials & secrets
	TestAdminEmail       = "admin@cadence.local"
	TestAdminPassword    = "AdminPass123!"
	TestAdminName        = "System Admin"
	TestMetricsToken     = "test_metrics_scrape_token_e2e_999"
	TestCookieSecret     = "e2e_super_secret_cookie_signing_key_1234567890"
	TestJWTAccessSecret  = "e2e_super_secret_jwt_access_key_1234567890"
	TestJWTRefreshSecret = "e2e_super_secret_jwt_refresh_key_1234567890"
	TestVAPIDPublicKey   = "BHK-KDYnjw1Z0O844tmUWbz3m7OT8faBNZ8P0se6qjeqqnuIhtJa4zU-XyFfre29bHMUCQaW6hw5Ck4R20H6ziQ"
	TestVAPIDPrivateKey  = "GWjhiHXmi8zODFg6GOGY4sN1SyAdkI8Kb-kalR15nfU"
	TestFrontendURL      = "http://localhost:3000"
)

var (
	testApp       *bootstrap.Application
	testServer    *httptest.Server
	mailpitAPIURL string
	rabbitAMQPURL string
	adminSeedHash string
)

func TestMain(m *testing.M) {
	os.Exit(runTestMain(m))
}

func runTestMain(m *testing.M) int {
	startupCtx, startupCancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer startupCancel()

	appCtx, appCancel := context.WithCancel(context.Background())

	var cleanups []func()
	registerCleanup := func(fn func()) {
		cleanups = append(cleanups, fn)
	}

	defer func() {
		// 1. Signal background worker goroutines (mailer, audit, push) to stop first
		appCancel()

		// 2. Run registered cleanups in reverse (LIFO) order with a dedicated timeout
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}()

	// Pre-compute admin bcrypt hash once for fast state resets
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(TestAdminPassword), bcrypt.MinCost)
	if err != nil {
		log.Printf("[E2E HARNESS] Failed to generate admin bcrypt hash: %v", err)
		return 1
	}
	adminSeedHash = string(hashBytes)

	// 1. Start PostgreSQL 16 container
	log.Printf("[E2E HARNESS] Starting PostgreSQL container (%s)...", postgresImage)
	pgContainer, err := tcpostgres.Run(
		startupCtx,
		postgresImage,
		tcpostgres.WithDatabase("cadence_e2e"),
		tcpostgres.WithUsername("cadence_user"),
		tcpostgres.WithPassword("cadence_pass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if pgContainer != nil {
		registerCleanup(func() {
			termCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if err := pgContainer.Terminate(termCtx); err != nil {
				log.Printf("[E2E HARNESS] Failed to terminate Postgres container: %v", err)
			}
		})
	}
	if err != nil {
		log.Printf("[E2E HARNESS] Failed to start Postgres container: %v", err)
		return 1
	}

	dbURL, err := pgContainer.ConnectionString(startupCtx, "sslmode=disable")
	if err != nil {
		log.Printf("[E2E HARNESS] Failed to get Postgres connection string: %v", err)
		return 1
	}

	// Apply database migrations
	if err := applyMigrations(dbURL); err != nil {
		log.Printf("[E2E HARNESS] Failed to apply database migrations: %v", err)
		return 1
	}
	log.Println("[E2E HARNESS] PostgreSQL migrations applied successfully")

	// 2. Start Redis 7 container
	log.Printf("[E2E HARNESS] Starting Redis container (%s)...", redisImage)
	redisContainer, err := tcredis.Run(
		startupCtx,
		redisImage,
	)
	if redisContainer != nil {
		registerCleanup(func() {
			termCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if err := redisContainer.Terminate(termCtx); err != nil {
				log.Printf("[E2E HARNESS] Failed to terminate Redis container: %v", err)
			}
		})
	}
	if err != nil {
		log.Printf("[E2E HARNESS] Failed to start Redis container: %v", err)
		return 1
	}

	redisURL, err := redisContainer.ConnectionString(startupCtx)
	if err != nil {
		log.Printf("[E2E HARNESS] Failed to get Redis connection string: %v", err)
		return 1
	}

	// 3. Start RabbitMQ 4 container
	log.Printf("[E2E HARNESS] Starting RabbitMQ container (%s)...", rabbitmqImage)
	rabbitContainer, err := tcrabbitmq.Run(
		startupCtx,
		rabbitmqImage,
		tcrabbitmq.WithAdminUsername("guest"),
		tcrabbitmq.WithAdminPassword("guest"),
	)
	if rabbitContainer != nil {
		registerCleanup(func() {
			termCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if err := rabbitContainer.Terminate(termCtx); err != nil {
				log.Printf("[E2E HARNESS] Failed to terminate RabbitMQ container: %v", err)
			}
		})
	}
	if err != nil {
		log.Printf("[E2E HARNESS] Failed to start RabbitMQ container: %v", err)
		return 1
	}

	rabbitAMQPURL, err = rabbitContainer.AmqpURL(startupCtx)
	if err != nil {
		log.Printf("[E2E HARNESS] Failed to get RabbitMQ AMQP URL: %v", err)
		return 1
	}

	// 4. Start Mailpit container (SMTP 1025/tcp + HTTP REST API 8025/tcp)
	log.Printf("[E2E HARNESS] Starting Mailpit container (%s)...", mailpitImage)
	mailpitReq := testcontainers.ContainerRequest{
		Image:        mailpitImage,
		ExposedPorts: []string{"1025/tcp", "8025/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("1025/tcp"),
			wait.ForHTTP("/api/v1/info").WithPort("8025/tcp").WithStatusCodeMatcher(func(status int) bool {
				return status == http.StatusOK
			}),
		).WithDeadline(60 * time.Second),
	}
	mailpitContainer, err := testcontainers.GenericContainer(startupCtx, testcontainers.GenericContainerRequest{
		ContainerRequest: mailpitReq,
		Started:          true,
	})
	if mailpitContainer != nil {
		registerCleanup(func() {
			termCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if err := mailpitContainer.Terminate(termCtx); err != nil {
				log.Printf("[E2E HARNESS] Failed to terminate Mailpit container: %v", err)
			}
		})
	}
	if err != nil {
		log.Printf("[E2E HARNESS] Failed to start Mailpit container: %v", err)
		return 1
	}

	mpHost, err := mailpitContainer.Host(startupCtx)
	if err != nil {
		log.Printf("[E2E HARNESS] Failed to resolve Mailpit host: %v", err)
		return 1
	}
	// Normalize loopback host to "localhost" so mailer.NewMailer selects mail.NoTLS automatically
	// when the application runs in-process on the host machine.
	smtpHost := mpHost
	if smtpHost == "127.0.0.1" || smtpHost == "::1" || smtpHost == "localhost" {
		smtpHost = "localhost"
	}

	smtpMappedPort, err := mailpitContainer.MappedPort(startupCtx, "1025/tcp")
	if err != nil {
		log.Printf("[E2E HARNESS] Failed to resolve Mailpit SMTP port: %v", err)
		return 1
	}
	httpMappedPort, err := mailpitContainer.MappedPort(startupCtx, "8025/tcp")
	if err != nil {
		log.Printf("[E2E HARNESS] Failed to resolve Mailpit HTTP port: %v", err)
		return 1
	}

	mailpitAPIURL = fmt.Sprintf("http://%s:%s", smtpHost, httpMappedPort.Port())
	log.Printf("[E2E HARNESS] Mailpit ready at SMTP=%s:%s, HTTP=%s", smtpHost, smtpMappedPort.Port(), mailpitAPIURL)

	// 5. Configure environment variables for bootstrap.NewApplication
	envVars := map[string]string{
		"PORT":                   "0",
		"APP_ENV":                "test",
		"FRONTEND_URL":           TestFrontendURL,
		"DATABASE_URL":           dbURL,
		"REDIS_URL":              redisURL,
		"RABBITMQ_URL":           rabbitAMQPURL,
		"SMTP_HOST":              smtpHost,
		"SMTP_PORT":              smtpMappedPort.Port(),
		"SMTP_USERNAME":          "",
		"SMTP_PASSWORD":          "",
		"SMTP_FROM_EMAIL":        "no-reply@cadence.local",
		"SMTP_FROM_NAME":         "Cadence E2E",
		"SMTP_SSL":               "false",
		"JWT_ACCESS_SECRET":      TestJWTAccessSecret,
		"JWT_REFRESH_SECRET":     TestJWTRefreshSecret,
		"JWT_ACCESS_EXPIRY":      "15m",
		"JWT_REFRESH_EXPIRY":     "168h",
		"COOKIE_SECRET":          TestCookieSecret,
		"COOKIE_DOMAIN":          "",
		"COOKIE_SECURE":          "false",
		"COOKIE_SAMESITE":        "lax",
		"ACCESS_COOKIE_EXPIRY":   "15m",
		"REFRESH_COOKIE_EXPIRY":  "168h",
		"SEED_ADMIN_EMAIL":       TestAdminEmail,
		"SEED_ADMIN_PASSWORD":    TestAdminPassword,
		"SEED_ADMIN_NAME":        TestAdminName,
		"VAPID_PUBLIC_KEY":       TestVAPIDPublicKey,
		"VAPID_PRIVATE_KEY":      TestVAPIDPrivateKey,
		"VAPID_SUBJECT":          "mailto:admin@cadence.local",
		"METRICS_SCRAPE_TOKEN":   TestMetricsToken,
		"INTERNAL_EMAIL_DOMAINS": "internal.example",
	}
	for k, v := range envVars {
		if err := os.Setenv(k, v); err != nil {
			log.Printf("[E2E HARNESS] Failed to set env %s: %v", k, err)
			return 1
		}
	}

	// 6. Boot Cadence Application and HTTP test server
	app, err := bootstrap.NewApplication(appCtx)
	if err != nil {
		log.Printf("[E2E HARNESS] Failed to initialize Cadence application: %v", err)
		return 1
	}
	testApp = app
	registerCleanup(func() {
		if testApp.MailQueue != nil {
			_ = testApp.MailQueue.Close()
		}
		if testApp.AuditQueue != nil {
			_ = testApp.AuditQueue.Close()
		}
		if testApp.NotificationQueue != nil {
			_ = testApp.NotificationQueue.Close()
		}
		if testApp.Mailer != nil {
			_ = testApp.Mailer.Close()
		}
		if testApp.Redis != nil {
			_ = testApp.Redis.Close()
		}
		if testApp.DB != nil {
			testApp.DB.Close()
		}
	})

	testServer = httptest.NewServer(testApp.Router)
	registerCleanup(func() {
		testServer.Close()
	})

	log.Printf("[E2E HARNESS] Cadence httptest.Server listening at %s", testServer.URL)

	return m.Run()
}

func applyMigrations(dbURL string) error {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New("failed to locate harness_test.go path via runtime.Caller")
	}
	migrationsDir := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "migrations"))

	m, err := migrate.New("file://"+migrationsDir, dbURL)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations up: %w", err)
	}
	return nil
}

// ResetState deterministically purges RabbitMQ queues, truncates PostgreSQL tables,
// flushes Redis keys, clears Mailpit messages, and seeds the root administrator.
func ResetState(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Purge RabbitMQ queues so no residual jobs leak into the next test
	purgeRabbitQueues(t)

	// Allow any in-flight consumer callbacks to settle before truncating tables
	time.Sleep(50 * time.Millisecond)

	// 2. Truncate all Postgres tables
	truncateSQL := `
		TRUNCATE TABLE
			notification_recipients,
			notifications,
			push_subscriptions,
			judging_scores,
			judging_slots,
			judging_panel_judges,
			judging_panels,
			judging_criteria,
			judging_rounds,
			app_settings,
			scan_events,
			scan_sessions,
			tickets,
			audit_logs,
			password_reset_tokens,
			refresh_tokens,
			users,
			teams
		RESTART IDENTITY CASCADE;
	`
	if _, err := testApp.DB.Exec(ctx, truncateSQL); err != nil {
		t.Fatalf("[ResetState] failed to truncate PostgreSQL tables: %v", err)
	}

	// 3. Flush Redis (rate limits, login guards, user blocklist)
	if err := testApp.Redis.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("[ResetState] failed to flush Redis: %v", err)
	}

	// 4. Clear Mailpit messages
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, mailpitAPIURL+"/api/v1/messages", nil)
	if err != nil {
		t.Fatalf("[ResetState] failed to build Mailpit delete request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("[ResetState] failed to clear Mailpit messages: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("[ResetState] unexpected status clearing Mailpit messages: %d", resp.StatusCode)
	}

	// 5. Seed deterministic root administrator account
	insertAdminSQL := `
		INSERT INTO users (email, password_hash, name, role, is_password_configured, is_active)
		VALUES ($1, $2, $3, 'admin', TRUE, TRUE)
	`
	if _, err := testApp.DB.Exec(ctx, insertAdminSQL, TestAdminEmail, adminSeedHash, TestAdminName); err != nil {
		t.Fatalf("[ResetState] failed to seed root admin user: %v", err)
	}
}

func purgeRabbitQueues(t *testing.T) {
	t.Helper()
	conn, err := amqp.Dial(rabbitAMQPURL)
	if err != nil {
		t.Fatalf("[ResetState] failed to connect to RabbitMQ for queue purge: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("[ResetState] failed to open RabbitMQ channel for queue purge: %v", err)
	}
	defer ch.Close()

	queues := []string{
		"cadence_emails",
		"cadence_audit_logs",
		"cadence_push_notifications",
	}
	for _, q := range queues {
		if _, err := ch.QueuePurge(q, false); err != nil {
			t.Fatalf("[ResetState] failed to purge RabbitMQ queue %q: %v", q, err)
		}
	}
}

func TestStage1_HarnessAndHealth(t *testing.T) {
	ResetState(t)

	anon := NewActor(t, "anon", "")

	// 1. Verify /health endpoint
	var healthResp map[string]string
	anon.DoJSON(t, http.MethodGet, "/health", nil, http.StatusOK, &healthResp)
	if healthResp["status"] != "ok" {
		t.Fatalf("expected health status 'ok', got %q", healthResp["status"])
	}
	if healthResp["env"] != "test" {
		t.Fatalf("expected env 'test', got %q", healthResp["env"])
	}
	if healthResp["postgres"] != "up" {
		t.Fatalf("expected postgres 'up', got %q", healthResp["postgres"])
	}
	if healthResp["redis"] != "up" {
		t.Fatalf("expected redis 'up', got %q", healthResp["redis"])
	}
	if healthResp["rabbitmq"] != "up" {
		t.Fatalf("expected rabbitmq 'up', got %q", healthResp["rabbitmq"])
	}

	// 2. Verify /metrics rejects unauthenticated scrape requests
	resp, body := anon.DoRaw(t, http.MethodGet, "/metrics", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized on /metrics without token, got %d (body: %s)", resp.StatusCode, string(body))
	}

	// 3. Verify /metrics succeeds with valid Bearer scrape token
	resp, body = anon.DoRaw(t, http.MethodGet, "/metrics", nil, map[string]string{
		"Authorization": "Bearer " + TestMetricsToken,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on /metrics with valid token, got %d (body: %s)", resp.StatusCode, string(body))
	}
}
