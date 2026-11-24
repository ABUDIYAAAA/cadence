package mailer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	DefaultEmailQueue = "cadence_emails"

	// maxDeliveryRetries is how many times a failed email is retried before being dropped.
	maxDeliveryRetries = 3
	retryCountHeader   = "x-retry-count"
)

var (
	ErrNilMailer = errors.New("mailer instance cannot be nil")
)

// Job represents an email task serialized into RabbitMQ.
type Job struct {
	TicketID    string         `json:"ticket_id,omitempty"`
	To          string         `json:"to"`
	Subject     string         `json:"subject"`
	Template    string         `json:"template,omitempty"`
	HTMLBody    string         `json:"html_body,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
	Attachments []Attachment   `json:"attachments,omitempty"`
}

// DeliveryCallback is invoked when a ticket email job is either delivered via SMTP or permanently dropped.
type DeliveryCallback func(ctx context.Context, ticketID string, delivered bool, errMsg string)

// Queue manages RabbitMQ publishing and background consumer goroutines.
type Queue struct {
	rabbitURL  string
	conn       *amqp.Connection
	pubCh      *amqp.Channel
	pubMu      sync.Mutex
	mailer     *Mailer
	queueName  string
	mu         sync.RWMutex
	onDelivery DeliveryCallback
}

// SetDeliveryCallback registers a hook to record ticket email delivery success or permanent failure.
func (q *Queue) SetDeliveryCallback(cb DeliveryCallback) {
	if q == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onDelivery = cb
}

func (q *Queue) notifyDelivery(ctx context.Context, ticketID string, delivered bool, errMsg string) {
	if q == nil || ticketID == "" {
		return
	}
	q.mu.RLock()
	cb := q.onDelivery
	q.mu.RUnlock()
	if cb != nil {
		cb(ctx, ticketID, delivered, errMsg)
	}
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
		rabbitURL: rabbitURL,
		conn:      conn,
		pubCh:     pubCh,
		mailer:    mailer,
		queueName: DefaultEmailQueue,
	}, nil
}

// ensurePublishChannelLocked reopens the AMQP connection or publish channel if closed. Caller must hold q.pubMu.
func (q *Queue) ensurePublishChannelLocked() error {
	if q.conn == nil || q.conn.IsClosed() {
		if q.rabbitURL == "" {
			return errors.New("rabbitmq connection closed")
		}
		conn, err := amqp.Dial(q.rabbitURL)
		if err != nil {
			return fmt.Errorf("reconnect to rabbitmq: %w", err)
		}
		q.conn = conn
		q.pubCh = nil
	}
	if q.pubCh == nil || q.pubCh.IsClosed() {
		ch, err := q.conn.Channel()
		if err != nil {
			return fmt.Errorf("reopen rabbitmq channel: %w", err)
		}
		_, _ = ch.QueueDeclare(q.queueName, true, false, false, false, nil)
		q.pubCh = ch
	}
	return nil
}

// Enqueue publishes an email job to RabbitMQ safely under pubMu.
func (q *Queue) Enqueue(ctx context.Context, job Job) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal email job: %w", err)
	}

	q.pubMu.Lock()
	defer q.pubMu.Unlock()

	if err := q.ensurePublishChannelLocked(); err != nil {
		return err
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

// SendTicketEmail enqueues an entry pass email whose body shows the QR code (the ticket code itself is never included).
func (q *Queue) SendTicketEmail(ctx context.Context, to, name, eventName string, qrPNG []byte) error {
	return q.SendCustomTicketEmail(ctx, "", to, name, eventName, "", "", qrPNG)
}

// SendCustomTicketEmail enqueues an entry pass email with an optional custom subject and pre-sanitized HTML body,
// attaching the QR code PNG as a standard file attachment (not inline).
func (q *Queue) SendCustomTicketEmail(
	ctx context.Context,
	ticketID, to, name, eventName, customSubject, renderedHTML string,
	qrPNG []byte,
) error {
	subject := strings.TrimSpace(customSubject)
	if subject == "" {
		subject = fmt.Sprintf("Your entry pass for %s", eventName)
	}

	job := Job{
		TicketID: ticketID,
		To:       to,
		Subject:  subject,
		Attachments: []Attachment{
			{
				Filename:    "ticket_qr.png",
				ContentType: "image/png",
				Data:        qrPNG,
				Inline:      false,
			},
		},
	}

	if strings.TrimSpace(renderedHTML) != "" {
		job.HTMLBody = renderedHTML
	} else {
		job.Template = "ticket"
		job.Data = map[string]any{
			"Name":      name,
			"EventName": eventName,
		}
	}

	return q.Enqueue(ctx, job)
}

// StartConsumers spins up a pool of background goroutines to consume and dispatch email jobs.
func (q *Queue) StartConsumers(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
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
					var err error
					if strings.TrimSpace(job.HTMLBody) != "" {
						err = q.mailer.SendHTMLWithAttachments(sendCtx, job.To, job.Subject, job.HTMLBody, job.Attachments)
					} else {
						err = q.mailer.SendWithAttachments(sendCtx, job.To, job.Subject, job.Template, job.Data, job.Attachments)
					}
					sendCancel()

					if err != nil {
						log.Printf("[MAILER WORKER %d] Failed to deliver email to %s: %v", workerID, job.To, err)
						q.handleFailure(ch, delivery, job, err, workerID)
					} else {
						log.Printf("[MAILER WORKER %d] Email delivered successfully to %s (%s)", workerID, job.To, job.Subject)
						if job.TicketID != "" {
							cbCtx, cbCancel := context.WithTimeout(context.Background(), 5*time.Second)
							q.notifyDelivery(cbCtx, job.TicketID, true, "")
							cbCancel()
						}
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

// IsHealthy returns true if the RabbitMQ connection is active and not closed.
func (q *Queue) IsHealthy() bool {
	return q != nil && q.conn != nil && !q.conn.IsClosed()
}

// Close gracefully terminates open channels and connection.
func (q *Queue) Close() error {
	q.pubMu.Lock()
	defer q.pubMu.Unlock()
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

// handleFailure retries a failed job a bounded number of times by republishing to the tail of the queue without blocking the worker goroutine, then drops it.
func (q *Queue) handleFailure(ch *amqp.Channel, delivery amqp.Delivery, job Job, sendErr error, workerID int) {
	attempts := 0
	if v, ok := delivery.Headers[retryCountHeader]; ok {
		switch n := v.(type) {
		case int32:
			attempts = int(n)
		case int64:
			attempts = int(n)
		case int:
			attempts = n
		}
	}

	permanent := errors.Is(sendErr, ErrTemplateNotFound) || errors.Is(sendErr, ErrHeaderInjection)
	if permanent || attempts >= maxDeliveryRetries {
		log.Printf("[MAILER WORKER %d] Dropping email job after %d retries (permanent=%t): %v", workerID, attempts, permanent, sendErr)
		if job.TicketID != "" {
			cbCtx, cbCancel := context.WithTimeout(context.Background(), 5*time.Second)
			q.notifyDelivery(cbCtx, job.TicketID, false, sendErr.Error())
			cbCancel()
		}
		_ = delivery.Nack(false, false)
		return
	}

	// Republish to the tail of the queue with an incremented counter, then ack the original immediately (no blocking time.Sleep).
	headers := amqp.Table{}
	for k, v := range delivery.Headers {
		headers[k] = v
	}
	headers[retryCountHeader] = int32(attempts + 1)

	pubCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := ch.PublishWithContext(pubCtx, "", q.queueName, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Timestamp:    time.Now(),
		Headers:      headers,
		Body:         delivery.Body,
	})
	cancel()
	if err != nil {
		log.Printf("[MAILER WORKER %d] Failed to republish email job, requeueing as-is: %v", workerID, err)
		_ = delivery.Nack(false, true)
		return
	}
	_ = delivery.Ack(false)
}
