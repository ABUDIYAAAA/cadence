package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

func newTestAuditPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping audit DB test: DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	schemaName := fmt.Sprintf("test_audit_%d", time.Now().UnixNano())
	adminPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping audit DB test: cannot connect to Postgres: %v", err)
	}
	if err := adminPool.Ping(ctx); err != nil {
		adminPool.Close()
		t.Skipf("skipping audit DB test: Postgres unreachable: %v", err)
	}

	if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", schemaName)); err != nil {
		adminPool.Close()
		t.Fatalf("create test schema: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		adminPool.Close()
		t.Fatalf("parse db url: %v", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = make(map[string]string)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		_, _ = adminPool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
		adminPool.Close()
		t.Fatalf("create schema-scoped pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		_, _ = adminPool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
		adminPool.Close()
	})

	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var upFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)
	for _, name := range upFiles {
		var num int
		if _, err := fmt.Sscanf(name, "%06d_", &num); err == nil && num > 4 {
			continue
		}
		sqlBytes, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}

	return pool
}

func TestOversizedEmailValidationAndAuditSanitization(t *testing.T) {
	t.Run("auth DTO rejects oversized email (>254 chars) on login and forgot-password", func(t *testing.T) {
		longEmail := strings.Repeat("a", 250) + "@example.com" // 262 chars
		loginErrs := utils.ValidateStruct(auth.LoginRequest{
			Email:    longEmail,
			Password: "password123",
		})
		if len(loginErrs) == 0 {
			t.Fatal("expected validation error for >254 char email in LoginRequest, got none")
		}

		forgotErrs := utils.ValidateStruct(auth.ForgotPasswordRequest{
			Email: longEmail,
		})
		if len(forgotErrs) == 0 {
			t.Fatal("expected validation error for >254 char email in ForgotPasswordRequest, got none")
		}
	})

	t.Run("SanitizeEvent truncates oversized fields, handles multibyte UTF-8 without corruption, records metadata, and strips NUL bytes", func(t *testing.T) {
		longEmail := strings.Repeat("x", 350) + "\x00@example.com"
		longDevice := strings.Repeat("d", 100)
		longIP := strings.Repeat("1", 80)
		// Multibyte unicode string (each emoji is 4 bytes). Truncating must not break mid-rune.
		multibyteDesc := strings.Repeat("🚀", 600) // 600 runes = 2400 bytes, max rune count is 2048, max byte is 8192

		invalidUUID := "not-a-valid-uuid"
		ev, notes := SanitizeEventWithReport(Event{
			ActorID:     &invalidUUID,
			ActorEmail:  &longEmail,
			Action:      strings.Repeat("act", 50),
			TargetType:  strings.Repeat("tgt", 30),
			Description: multibyteDesc + "\x00after-null",
			DeviceID:    &longDevice,
			IPAddress:   &longIP,
		})

		if ev.ActorID != nil {
			t.Fatalf("expected malformed actor_id to be rejected (nil), got %v", *ev.ActorID)
		}
		if len(*ev.ActorEmail) != 255 {
			t.Fatalf("expected ActorEmail truncated to 255, got %d", len(*ev.ActorEmail))
		}
		if strings.Contains(*ev.ActorEmail, "\x00") || strings.Contains(ev.Description, "\x00") {
			t.Fatal("expected NUL bytes to be stripped from sanitized event")
		}
		if len(*ev.DeviceID) != 64 {
			t.Fatalf("expected DeviceID truncated to 64, got %d", len(*ev.DeviceID))
		}
		if len(*ev.IPAddress) != 45 {
			t.Fatalf("expected IPAddress truncated to 45, got %d", len(*ev.IPAddress))
		}
		if len(notes) == 0 {
			t.Fatal("expected forensic notes for sanitized/truncated fields")
		}
		if ev.Metadata["_audit_sanitized"] != true {
			t.Fatal("expected _audit_sanitized metadata flag on sanitized event")
		}
	})
}

func TestAuditConsumer_OversizedAndPoisonMessagesDoNotStallQueue(t *testing.T) {
	pool := newTestAuditPool(t)
	repo := NewRepository(pool)

	_ = godotenv.Load("../../../.env")
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		t.Skip("skipping RabbitMQ audit consumer test: RABBITMQ_URL not set")
	}

	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		t.Skipf("skipping RabbitMQ audit consumer test: cannot connect to RabbitMQ: %v", err)
	}
	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		t.Fatalf("open rabbitmq channel: %v", err)
	}

	testQueue := fmt.Sprintf("test_audit_logs_%d", time.Now().UnixNano())
	testDLQ := testQueue + "_dlq"

	if _, err := pubCh.QueueDeclare(testQueue, true, false, false, false, nil); err != nil {
		t.Fatalf("declare test queue: %v", err)
	}
	if _, err := pubCh.QueueDeclare(testDLQ, true, false, false, false, nil); err != nil {
		t.Fatalf("declare test dlq: %v", err)
	}

	q := &Queue{
		conn:      conn,
		pubCh:     pubCh,
		repo:      repo,
		queueName: testQueue,
		dlqName:   testDLQ,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	t.Cleanup(func() {
		_, _ = pubCh.QueueDelete(testQueue, false, false, false)
		_, _ = pubCh.QueueDelete(testDLQ, false, false, false)
		_ = q.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.StartConsumers(ctx, 3)

	// 1. Send 35 oversized-email events through q.Record AND malformed raw messages directly to the queue
	for i := 0; i < 35; i++ {
		oversizedEmail := fmt.Sprintf("%s-%d@example.com", strings.Repeat("p", 260), i)
		q.Record(ctx, Event{
			ActorEmail:  &oversizedEmail,
			Action:      ActionAuthLoginFailure,
			TargetType:  TargetAuth,
			Description: "Oversized email attempt",
		})
	}

	// Also inject 35 raw malformed JSON payloads directly into the queue to simulate poison messages
	for i := 0; i < 35; i++ {
		_ = pubCh.PublishWithContext(ctx, "", testQueue, false, false, amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte("{malformed-json"),
		})
	}

	// 2. Now publish a legitimate login success audit event and verify it is persisted promptly
	legitEmail := "legit.user@cadence.local"
	rawPayload, _ := json.Marshal(Event{
		ActorEmail:  &legitEmail,
		Action:      ActionAuthLoginSuccess,
		TargetType:  TargetAuth,
		Description: "User legit.user@cadence.local logged in successfully",
	})
	if err := pubCh.PublishWithContext(ctx, "", testQueue, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        rawPayload,
	}); err != nil {
		t.Fatalf("publish legitimate audit event: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var persisted int
	for time.Now().Before(deadline) {
		_ = pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_logs WHERE actor_email = $1 AND action = $2`,
			legitEmail, ActionAuthLoginSuccess,
		).Scan(&persisted)
		if persisted == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if persisted != 1 {
		t.Fatalf("expected legitimate audit event to be persisted despite 70 preceding oversized/poison messages, got count=%d", persisted)
	}
}
