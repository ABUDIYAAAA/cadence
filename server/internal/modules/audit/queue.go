package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"devclubxnst.com/cadence/pkg/amqpqueue"
)

var (
	ErrDLQUnavailable   = amqpqueue.ErrDLQUnavailable
	ErrPublishUnconfirm = amqpqueue.ErrPublishUnconfirm
)

const (
	DefaultAuditQueue       = "cadence_audit_logs"
	DefaultAuditDLQ         = "cadence_audit_logs_dlq"
	MaxAuditDeliveryRetries = 3
	AuditRetryHeader        = amqpqueue.DefaultRetryHeader
)

// Queue manages RabbitMQ publishing and background consumer goroutines for audit events.
type Queue struct {
	broker amqpqueue.Broker
	repo   *Repository
	logger *slog.Logger
}

// NewQueue connects to RabbitMQ and declares the durable audit logs queue and dead-letter queue.
func NewQueue(rabbitURL string, repo *Repository, l *slog.Logger) (*Queue, error) {
	return newQueueWithNames(rabbitURL, DefaultAuditQueue, DefaultAuditDLQ, repo, l)
}

func newQueueWithNames(rabbitURL, queueName, dlqName string, repo *Repository, l *slog.Logger) (*Queue, error) {
	if l == nil {
		l = slog.Default()
	}
	broker, err := amqpqueue.NewAMQPBroker(amqpqueue.Config{
		URL:            rabbitURL,
		QueueName:      queueName,
		DLQName:        dlqName,
		ConsumerPrefix: "audit-worker",
		Prefetch:       10,
		MaxRetries:     MaxAuditDeliveryRetries,
		RetryHeader:    AuditRetryHeader,
		PublishTimeout: 2 * time.Second,
		Logger:         l,
	})
	if err != nil {
		return nil, fmt.Errorf("init audit amqp broker: %w", err)
	}
	return NewQueueWithBroker(broker, repo, l), nil
}

// NewQueueWithBroker builds an audit Queue backed by any amqpqueue.Broker (e.g. MemoryBroker in tests).
func NewQueueWithBroker(broker amqpqueue.Broker, repo *Repository, l *slog.Logger) *Queue {
	if l == nil {
		l = slog.Default()
	}
	return &Queue{
		broker: broker,
		repo:   repo,
		logger: l,
	}
}

// Record captures an audit event asynchronously by publishing it to RabbitMQ.
// It returns immediately (<1ms) to eliminate latency in HTTP request handlers.
func (q *Queue) Record(ctx context.Context, event Event) {
	// A nil queue means RabbitMQ was unreachable at startup. Services hold it behind an interface,
	// where their own `== nil` guards cannot see it, so audit becomes a no-op instead of a panic.
	if q == nil || q.broker == nil {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	var sanitizedNotes []string
	event, sanitizedNotes = SanitizeEventWithReport(event)
	if len(sanitizedNotes) > 0 && q.logger != nil {
		q.logger.Warn("Audit event fields sanitized or truncated before publish",
			slog.String("action", event.Action),
			slog.String("target_type", event.TargetType),
			slog.Any("sanitized_fields", sanitizedNotes),
		)
	}

	payload, err := json.Marshal(event)
	if err != nil {
		q.logger.Error("Failed to marshal audit event", slog.String("action", event.Action), slog.Any("error", err))
		return
	}

	pubCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := q.broker.Publish(pubCtx, payload, amqpqueue.WithTimestamp(event.Timestamp)); err != nil {
		q.logger.Error("Failed to publish audit event to RabbitMQ", slog.String("action", event.Action), slog.Any("error", err))
	}
}

// StartConsumers launches background worker goroutines to persist audit logs into PostgreSQL.
func (q *Queue) StartConsumers(ctx context.Context, workers int) {
	if q == nil || q.broker == nil {
		return
	}
	q.broker.StartConsumers(ctx, workers, q.handleDelivery)
}

func (q *Queue) handleDelivery(_ context.Context, msg amqpqueue.Delivery) error {
	var event Event
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		return amqpqueue.Permanent(fmt.Errorf("malformed audit event payload: %w", err))
	}

	insertCtx, insertCancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := q.repo.InsertAuditLog(insertCtx, event)
	insertCancel()

	if err != nil {
		if IsPermanentAuditInsertError(err) {
			return amqpqueue.Permanent(err)
		}
		return err
	}
	return nil
}

// IsHealthy returns true if the RabbitMQ connection is active and not closed.
func (q *Queue) IsHealthy() bool {
	return q != nil && q.broker != nil && q.broker.IsHealthy()
}

// Close closes the underlying RabbitMQ broker.
func (q *Queue) Close() error {
	if q == nil || q.broker == nil {
		return nil
	}
	if err := q.broker.Close(); err != nil && !errors.Is(err, amqpqueue.ErrBrokerClosed) {
		return fmt.Errorf("close audit queue: %w", err)
	}
	return nil
}
