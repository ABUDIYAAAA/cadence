package mailer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"devclubxnst.com/cadence/pkg/amqpqueue"
)

const (
	DefaultEmailQueue = "cadence_emails"

	// maxDeliveryRetries is how many times a failed email is retried before being dropped.
	maxDeliveryRetries = 3
	retryCountHeader   = amqpqueue.DefaultRetryHeader
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
	broker     amqpqueue.Broker
	mailer     *Mailer
	logger     *slog.Logger
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

	logger := slog.Default().With(slog.String("component", "mailer"))
	q := &Queue{
		mailer: mailer,
		logger: logger,
	}

	broker, err := amqpqueue.NewAMQPBroker(amqpqueue.Config{
		URL:               rabbitURL,
		QueueName:         DefaultEmailQueue,
		ConsumerPrefix:    "mailer-worker",
		Prefetch:          1,
		MaxRetries:        maxDeliveryRetries,
		RetryHeader:       retryCountHeader,
		PublishTimeout:    5 * time.Second,
		Logger:            logger,
		OnTerminalFailure: q.handleTerminalFailure,
	})
	if err != nil {
		return nil, fmt.Errorf("init mailer amqp broker: %w", err)
	}

	q.broker = broker
	return q, nil
}

// NewQueueWithBroker builds a mailer Queue backed by any amqpqueue.Broker (e.g. MemoryBroker in tests).
func NewQueueWithBroker(broker amqpqueue.Broker, mailer *Mailer) (*Queue, error) {
	if mailer == nil {
		return nil, ErrNilMailer
	}
	return &Queue{
		broker: broker,
		mailer: mailer,
		logger: slog.Default().With(slog.String("component", "mailer")),
	}, nil
}

// Enqueue publishes an email job to RabbitMQ.
func (q *Queue) Enqueue(ctx context.Context, job Job) error {
	if q == nil || q.broker == nil {
		return amqpqueue.ErrBrokerClosed
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal email job: %w", err)
	}

	if err := q.broker.Publish(ctx, payload); err != nil {
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
	if q == nil || q.broker == nil {
		return
	}
	q.broker.StartConsumers(ctx, workers, q.handleDelivery)
}

func (q *Queue) handleDelivery(_ context.Context, msg amqpqueue.Delivery) error {
	var job Job
	if err := json.Unmarshal(msg.Body, &job); err != nil {
		return amqpqueue.Permanent(fmt.Errorf("parse email job: %w", err))
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
		q.logger.Warn("Failed to deliver email",
			slog.String("to", job.To),
			slog.Int("attempt", msg.Attempt),
			slog.Any("error", err),
		)
		if errors.Is(err, ErrTemplateNotFound) || errors.Is(err, ErrHeaderInjection) {
			return amqpqueue.Permanent(err)
		}
		return err
	}

	q.logger.Info("Email delivered successfully",
		slog.String("to", job.To),
		slog.String("subject", job.Subject),
	)
	if job.TicketID != "" {
		cbCtx, cbCancel := context.WithTimeout(context.Background(), 5*time.Second)
		q.notifyDelivery(cbCtx, job.TicketID, true, "")
		cbCancel()
	}
	return nil
}

func (q *Queue) handleTerminalFailure(ctx context.Context, msg amqpqueue.Delivery, err error, _ bool) {
	var job Job
	if unmarshalErr := json.Unmarshal(msg.Body, &job); unmarshalErr != nil || job.TicketID == "" {
		return
	}
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	q.notifyDelivery(ctx, job.TicketID, false, errMsg)
}

// IsHealthy returns true if the RabbitMQ connection is active and not closed.
func (q *Queue) IsHealthy() bool {
	return q != nil && q.broker != nil && q.broker.IsHealthy()
}

// Close gracefully terminates open channels and connection.
func (q *Queue) Close() error {
	if q == nil || q.broker == nil {
		return nil
	}
	if err := q.broker.Close(); err != nil && !errors.Is(err, amqpqueue.ErrBrokerClosed) {
		return fmt.Errorf("close rabbitmq queue: %w", err)
	}
	return nil
}
