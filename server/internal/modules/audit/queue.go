package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	ErrDLQUnavailable   = errors.New("audit dead-letter queue unavailable or unconfirmed")
	ErrPublishUnconfirm = errors.New("broker nacked or did not confirm message publish")
)

const (
	DefaultAuditQueue       = "cadence_audit_logs"
	DefaultAuditDLQ         = "cadence_audit_logs_dlq"
	MaxAuditDeliveryRetries = 3
	AuditRetryHeader        = "x-retry-count"
)

// Queue manages RabbitMQ publishing and background consumer goroutines for audit events.
type Queue struct {
	conn      *amqp.Connection
	pubCh     *amqp.Channel
	pubMu     sync.Mutex
	repo      *Repository
	queueName string
	dlqName   string
	logger    *slog.Logger
}

// NewQueue connects to RabbitMQ and declares the durable audit logs queue and dead-letter queue.
func NewQueue(rabbitURL string, repo *Repository, l *slog.Logger) (*Queue, error) {
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		return nil, fmt.Errorf("connect rabbitmq for audit: %w", err)
	}

	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel for audit: %w", err)
	}

	_, err = pubCh.QueueDeclare(
		DefaultAuditQueue,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		_ = pubCh.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare audit queue: %w", err)
	}

	_, err = pubCh.QueueDeclare(
		DefaultAuditDLQ,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		_ = pubCh.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare audit dlq: %w", err)
	}

	return &Queue{
		conn:      conn,
		pubCh:     pubCh,
		repo:      repo,
		queueName: DefaultAuditQueue,
		dlqName:   DefaultAuditDLQ,
		logger:    l,
	}, nil
}

// Record captures an audit event asynchronously by publishing it to RabbitMQ.
// It returns immediately (<1ms) to eliminate latency in HTTP request handlers.
func (q *Queue) Record(ctx context.Context, event Event) {
	// A nil queue means RabbitMQ was unreachable at startup. Services hold it behind an interface,
	// where their own `== nil` guards cannot see it, so audit becomes a no-op instead of a panic.
	if q == nil {
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

	q.pubMu.Lock()
	err = q.pubCh.PublishWithContext(
		pubCtx,
		"",          // exchange
		q.queueName, // routing key
		true,        // mandatory
		false,       // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Timestamp:    event.Timestamp,
			Body:         payload,
		},
	)
	q.pubMu.Unlock()
	if err != nil {
		q.logger.Error("Failed to publish audit event to RabbitMQ", slog.String("action", event.Action), slog.Any("error", err))
	}
}

// StartConsumers launches background worker goroutines to persist audit logs into PostgreSQL.
func (q *Queue) StartConsumers(ctx context.Context, workers int) {
	var wg sync.WaitGroup

	for i := 1; i <= workers; i++ {
		wg.Add(1)
		workerID := i

		go func() {
			defer wg.Done()

			ch, err := q.conn.Channel()
			if err != nil {
				q.logger.Error("Failed to open audit consumer channel", slog.Int("worker_id", workerID), slog.Any("error", err))
				return
			}
			defer ch.Close()

			// Enable publisher confirms on the consumer channel so DLQ and retry publishes
			// are guaranteed durable before the original delivery is acknowledged.
			if err := ch.Confirm(false); err != nil {
				q.logger.Error("Failed to enable publisher confirms on audit consumer channel", slog.Int("worker_id", workerID), slog.Any("error", err))
				return
			}

			_ = ch.Qos(10, 0, false) // Fetch up to 10 audit events for efficient ingestion

			msgs, err := ch.ConsumeWithContext(
				ctx,
				q.queueName,
				fmt.Sprintf("audit-worker-%d", workerID),
				false, // auto-ack disabled
				false,
				false,
				false,
				nil,
			)
			if err != nil {
				q.logger.Error("Failed to register audit consumer", slog.Int("worker_id", workerID), slog.Any("error", err))
				return
			}

			q.logger.Info("Audit log consumer ready", slog.Int("worker_id", workerID))

			for {
				select {
				case <-ctx.Done():
					return
				case delivery, ok := <-msgs:
					if !ok {
						return
					}
					q.handleDelivery(ctx, ch, delivery)
				}
			}
		}()
	}

	go func() {
		<-ctx.Done()
		wg.Wait()
		q.logger.Info("All audit consumer goroutines exited cleanly")
	}()
}

func (q *Queue) handleDelivery(ctx context.Context, ch *amqp.Channel, delivery amqp.Delivery) {
	var event Event
	if err := json.Unmarshal(delivery.Body, &event); err != nil {
		q.logger.Error("Malformed audit event payload; routing to DLQ", slog.Any("error", err))
		if dlqErr := q.publishToDLQ(ch, delivery.Body, extractRetryCount(delivery.Headers), err); dlqErr != nil {
			q.logger.Error("DLQ publish failed for malformed payload; retaining message in audit queue", slog.Any("error", dlqErr))
			select {
			case <-ctx.Done():
			case <-time.After(50 * time.Millisecond):
			}
			_ = delivery.Nack(false, true)
			return
		}
		_ = delivery.Ack(false)
		return
	}

	insertCtx, insertCancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := q.repo.InsertAuditLog(insertCtx, event)
	insertCancel()

	if err != nil {
		retryCount := extractRetryCount(delivery.Headers)
		if IsPermanentAuditInsertError(err) || retryCount >= MaxAuditDeliveryRetries {
			q.logger.Error("Routing poison audit event to DLQ",
				slog.String("action", event.Action),
				slog.Int("retry_count", retryCount),
				slog.Bool("permanent_sql_error", IsPermanentAuditInsertError(err)),
				slog.Any("error", err),
			)
			if dlqErr := q.publishToDLQ(ch, delivery.Body, retryCount, err); dlqErr != nil {
				// Never silently acknowledge or discard a message if the DLQ publish was not durably confirmed.
				q.logger.Error("DLQ publish failed; retaining original message in audit queue",
					slog.String("action", event.Action),
					slog.Any("dlq_error", dlqErr),
				)
				select {
				case <-ctx.Done():
				case <-time.After(50 * time.Millisecond):
				}
				_ = delivery.Nack(false, true)
				return
			}
			_ = delivery.Ack(false)
			return
		}

		nextRetry := retryCount + 1
		q.logger.Warn("Transient failure persisting audit log; scheduling bounded retry",
			slog.String("action", event.Action),
			slog.Int("attempt", nextRetry),
			slog.Int("max_retries", MaxAuditDeliveryRetries),
			slog.Any("error", err),
		)
		if pubErr := q.republishWithRetry(ch, delivery.Body, nextRetry, event.Timestamp); pubErr == nil {
			_ = delivery.Ack(false)
		} else {
			select {
			case <-ctx.Done():
			case <-time.After(50 * time.Millisecond):
			}
			_ = delivery.Nack(false, true)
		}
	} else {
		_ = delivery.Ack(false)
	}
}

// IsHealthy returns true if the RabbitMQ connection is active and not closed.
func (q *Queue) IsHealthy() bool {
	return q != nil && q.conn != nil && !q.conn.IsClosed()
}

// Close closes RabbitMQ channel and connection.
func (q *Queue) Close() error {
	var errs []error
	if q.pubCh != nil {
		if err := q.pubCh.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if q.conn != nil {
		if err := q.conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close audit queue: %v", errs)
	}
	return nil
}

func extractRetryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	raw, ok := headers[AuditRetryHeader]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func (q *Queue) republishWithRetry(ch *amqp.Channel, body []byte, retryCount int, ts time.Time) error {
	if ch == nil || q.queueName == "" {
		return ErrPublishUnconfirm
	}
	pubCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	confirmation, err := ch.PublishWithDeferredConfirmWithContext(
		pubCtx,
		"",
		q.queueName,
		true,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Timestamp:    ts,
			Headers: amqp.Table{
				AuditRetryHeader: int32(retryCount),
			},
			Body: body,
		},
	)
	if err != nil {
		return err
	}
	if confirmation != nil {
		ack, waitErr := confirmation.WaitContext(pubCtx)
		if waitErr != nil {
			return waitErr
		}
		if !ack {
			return ErrPublishUnconfirm
		}
	}
	return nil
}

func (q *Queue) publishToDLQ(ch *amqp.Channel, body []byte, retryCount int, cause error) error {
	if ch == nil || q.dlqName == "" {
		return ErrDLQUnavailable
	}
	pubCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	errMsg := ""
	if cause != nil {
		errMsg = cause.Error()
	}
	confirmation, err := ch.PublishWithDeferredConfirmWithContext(
		pubCtx,
		"",
		q.dlqName,
		true,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Timestamp:    time.Now(),
			Headers: amqp.Table{
				AuditRetryHeader: int32(retryCount),
				"x-error":        errMsg,
			},
			Body: body,
		},
	)
	if err != nil {
		return err
	}
	if confirmation != nil {
		ack, waitErr := confirmation.WaitContext(pubCtx)
		if waitErr != nil {
			return waitErr
		}
		if !ack {
			return ErrDLQUnavailable
		}
	}
	return nil
}
