package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const DefaultAuditQueue = "cadence_audit_logs"

// Queue manages RabbitMQ publishing and background consumer goroutines for audit events.
type Queue struct {
	conn      *amqp.Connection
	pubCh     *amqp.Channel
	repo      *Repository
	queueName string
	logger    *slog.Logger
}

// NewQueue connects to RabbitMQ and declares the durable audit logs queue.
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

	return &Queue{
		conn:      conn,
		pubCh:     pubCh,
		repo:      repo,
		queueName: DefaultAuditQueue,
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

	payload, err := json.Marshal(event)
	if err != nil {
		q.logger.Error("Failed to marshal audit event", slog.String("action", event.Action), slog.Any("error", err))
		return
	}

	pubCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

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

					var event Event
					if err := json.Unmarshal(delivery.Body, &event); err != nil {
						q.logger.Error("Malformed audit event payload", slog.Any("error", err))
						_ = delivery.Nack(false, false)
						continue
					}

					insertCtx, insertCancel := context.WithTimeout(context.Background(), 5*time.Second)
					err := q.repo.InsertAuditLog(insertCtx, event)
					insertCancel()

					if err != nil {
						q.logger.Error("Failed to persist audit log into PostgreSQL", slog.String("action", event.Action), slog.Any("error", err))
						_ = delivery.Nack(false, true) // requeue on temporary DB failure
					} else {
						_ = delivery.Ack(false)
					}
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
