package mailer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	DefaultEmailQueue = "cadence_emails"
)

var (
	ErrNilMailer = errors.New("mailer instance cannot be nil")
)

// Job represents an email task serialized into RabbitMQ.
type Job struct {
	To       string         `json:"to"`
	Subject  string         `json:"subject"`
	Template string         `json:"template"`
	Data     map[string]any `json:"data"`
}

// Queue manages RabbitMQ publishing and background consumer goroutines.
type Queue struct {
	conn      *amqp.Connection
	pubCh     *amqp.Channel
	mailer    *Mailer
	queueName string
}

// NewQueue creates and initializes RabbitMQ connection, channel, and declared queue.
func NewQueue(rabbitURL string, mailer *Mailer) (*Queue, error) {
	if mailer == nil {
		return nil, ErrNilMailer
	}

	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		return nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}

	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}

	// Declare durable queue
	_, err = pubCh.QueueDeclare(
		DefaultEmailQueue,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		_ = pubCh.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare rabbitmq queue: %w", err)
	}

	return &Queue{
		conn:      conn,
		pubCh:     pubCh,
		mailer:    mailer,
		queueName: DefaultEmailQueue,
	}, nil
}

// Enqueue publishes an email job to RabbitMQ.
func (q *Queue) Enqueue(ctx context.Context, job Job) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal email job: %w", err)
	}

	err = q.pubCh.PublishWithContext(
		ctx,
		"",          // exchange (direct to default)
		q.queueName, // routing key
		true,        // mandatory
		false,       // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Timestamp:    time.Now(),
			Body:         payload,
		},
	)
	if err != nil {
		return fmt.Errorf("publish email to rabbitmq: %w", err)
	}

	return nil
}

// SendSetupPasswordEmail enqueues a welcome/setup password email job.
func (q *Queue) SendSetupPasswordEmail(ctx context.Context, to, name, setupURL string, expiryHours int) error {
	return q.Enqueue(ctx, Job{
		To:       to,
		Subject:  "Welcome to Cadence - Set Up Your Password",
		Template: "setup_password",
		Data: map[string]any{
			"Name":        name,
			"SetupURL":    setupURL,
			"ExpiryHours": expiryHours,
		},
	})
}

// SendResetPasswordEmail enqueues a password reset email job.
func (q *Queue) SendResetPasswordEmail(ctx context.Context, to, name, resetURL string, expiryHours int) error {
	return q.Enqueue(ctx, Job{
		To:       to,
		Subject:  "Cadence - Password Reset Request",
		Template: "reset_password",
		Data: map[string]any{
			"Name":        name,
			"ResetURL":    resetURL,
			"ExpiryHours": expiryHours,
		},
	})
}

// StartConsumers spins up a pool of background goroutines to consume and dispatch email jobs.
func (q *Queue) StartConsumers(ctx context.Context, workers int) {
	var wg sync.WaitGroup

	for i := 1; i <= workers; i++ {
		wg.Add(1)
		workerID := i

		go func() {
			defer wg.Done()

			// Each worker goroutine maintains its own AMQP channel for thread safety
			ch, err := q.conn.Channel()
			if err != nil {
				log.Printf("[MAILER WORKER %d] Failed to create channel: %v", workerID, err)
				return
			}
			defer ch.Close()

			// Fair dispatch: fetch 1 message at a time
			if err := ch.Qos(1, 0, false); err != nil {
				log.Printf("[MAILER WORKER %d] Failed to set QoS: %v", workerID, err)
				return
			}

			msgs, err := ch.ConsumeWithContext(
				ctx,
				q.queueName,
				fmt.Sprintf("mailer-worker-%d", workerID),
				false, // auto-ack disabled for reliable delivery
				false, // exclusive
				false, // no-local
				false, // no-wait
				nil,
			)
			if err != nil {
				log.Printf("[MAILER WORKER %d] Failed to register consumer: %v", workerID, err)
				return
			}

			log.Printf("[MAILER WORKER %d] Ready and listening for email jobs", workerID)

			for {
				select {
				case <-ctx.Done():
					log.Printf("[MAILER WORKER %d] Shutting down", workerID)
					return
				case delivery, ok := <-msgs:
					if !ok {
						log.Printf("[MAILER WORKER %d] Delivery channel closed", workerID)
						return
					}

					var job Job
					if err := json.Unmarshal(delivery.Body, &job); err != nil {
						log.Printf("[MAILER WORKER %d] Failed to parse job: %v", workerID, err)
						_ = delivery.Nack(false, false) // discard malformed message
						continue
					}

					sendCtx, sendCancel := context.WithTimeout(context.Background(), 15*time.Second)
					err := q.mailer.Send(sendCtx, job.To, job.Subject, job.Template, job.Data)
					sendCancel()

					if err != nil {
						log.Printf("[MAILER WORKER %d] Failed to deliver email to %s: %v", workerID, job.To, err)
						_ = delivery.Nack(false, true) // requeue on delivery error
					} else {
						log.Printf("[MAILER WORKER %d] Email delivered successfully to %s (%s)", workerID, job.To, job.Subject)
						_ = delivery.Ack(false)
					}
				}
			}
		}()
	}

	go func() {
		<-ctx.Done()
		wg.Wait()
		log.Println("[MAILER] All consumer goroutines exited cleanly")
	}()
}

// Close gracefully terminates open channels and connection.
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
		return fmt.Errorf("close rabbitmq queue: %v", errs)
	}
	return nil
}
