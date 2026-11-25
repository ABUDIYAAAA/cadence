package amqpqueue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// AMQPBroker is a resilient RabbitMQ publisher and reconnecting consumer pool.
// It owns connection/channel recovery for both publishers and consumers,
// publisher confirms, bounded retries, and optional DLQ routing.
type AMQPBroker struct {
	cfg Config

	connMu sync.RWMutex
	conn   *amqp.Connection
	closed bool

	pubMu sync.Mutex
	pubCh *amqp.Channel
}

// NewAMQPBroker connects to RabbitMQ, declares the primary queue (and DLQ if
// configured), and enables publisher confirms on the publish channel.
func NewAMQPBroker(cfg Config) (*AMQPBroker, error) {
	cfg = cfg.withDefaults()
	if cfg.URL == "" {
		return nil, errors.New("amqpqueue: RabbitMQ URL is required")
	}
	if cfg.QueueName == "" {
		return nil, errors.New("amqpqueue: queue name is required")
	}

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("connect rabbitmq (%s): %w", cfg.QueueName, err)
	}

	pubCh, err := openPublishChannel(conn, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &AMQPBroker{
		cfg:   cfg,
		conn:  conn,
		pubCh: pubCh,
	}, nil
}

func openPublishChannel(conn *amqp.Connection, cfg Config) (*amqp.Channel, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open rabbitmq channel (%s): %w", cfg.QueueName, err)
	}
	if err := declareTopology(ch, cfg); err != nil {
		_ = ch.Close()
		return nil, err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("enable publisher confirms (%s): %w", cfg.QueueName, err)
	}
	return ch, nil
}

func declareTopology(ch *amqp.Channel, cfg Config) error {
	if _, err := ch.QueueDeclare(cfg.QueueName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %s: %w", cfg.QueueName, err)
	}
	if cfg.DLQName != "" {
		if _, err := ch.QueueDeclare(cfg.DLQName, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare dlq %s: %w", cfg.DLQName, err)
		}
	}
	return nil
}

// ensureConnection returns an open AMQP connection, re-dialing if closed.
// Serialized under connMu so concurrent publishers and reconnecting consumers
// share a single healthy connection instead of dialing redundantly.
func (b *AMQPBroker) ensureConnection() (*amqp.Connection, error) {
	b.connMu.RLock()
	if b.closed {
		b.connMu.RUnlock()
		return nil, ErrBrokerClosed
	}
	if b.conn != nil && !b.conn.IsClosed() {
		conn := b.conn
		b.connMu.RUnlock()
		return conn, nil
	}
	b.connMu.RUnlock()

	b.connMu.Lock()
	defer b.connMu.Unlock()

	if b.closed {
		return nil, ErrBrokerClosed
	}
	if b.conn != nil && !b.conn.IsClosed() {
		return b.conn, nil
	}
	if b.cfg.URL == "" {
		return nil, errors.New("rabbitmq connection closed")
	}

	conn, err := amqp.Dial(b.cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("reconnect rabbitmq (%s): %w", b.cfg.QueueName, err)
	}
	b.conn = conn
	return conn, nil
}

// ensurePublishChannelLocked returns an open publish channel with publisher confirms enabled.
// Caller must hold b.pubMu.
func (b *AMQPBroker) ensurePublishChannelLocked() (*amqp.Channel, error) {
	conn, err := b.ensureConnection()
	if err != nil {
		return nil, err
	}
	if b.pubCh == nil || b.pubCh.IsClosed() {
		ch, err := openPublishChannel(conn, b.cfg)
		if err != nil {
			return nil, err
		}
		b.pubCh = ch
	}
	return b.pubCh, nil
}

// Publish durably publishes a message to the configured queue with publisher confirms.
func (b *AMQPBroker) Publish(ctx context.Context, body []byte, opts ...PublishOption) error {
	if b == nil {
		return ErrBrokerClosed
	}
	po := resolvePublishOptions(opts)

	var headers amqp.Table
	if len(po.Headers) > 0 {
		headers = amqp.Table(cloneHeaders(po.Headers))
	}

	b.pubMu.Lock()
	defer b.pubMu.Unlock()

	ch, err := b.ensurePublishChannelLocked()
	if err != nil {
		return err
	}

	confirm, err := ch.PublishWithDeferredConfirmWithContext(
		ctx,
		"",
		b.cfg.QueueName,
		true,  // mandatory
		false, // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  po.ContentType,
			Timestamp:    po.Timestamp,
			Headers:      headers,
			Body:         body,
		},
	)
	if err != nil {
		// Force channel re-creation on the next publish attempt.
		_ = ch.Close()
		b.pubCh = nil
		return fmt.Errorf("publish to %s: %w", b.cfg.QueueName, err)
	}
	if confirm != nil {
		ack, waitErr := confirm.WaitContext(ctx)
		if waitErr != nil {
			return fmt.Errorf("await publish confirm (%s): %w", b.cfg.QueueName, waitErr)
		}
		if !ack {
			return ErrPublishUnconfirm
		}
	}
	return nil
}

// StartConsumers launches background worker goroutines that automatically
// reconnect with exponential backoff if RabbitMQ restarts or drops the channel.
func (b *AMQPBroker) StartConsumers(ctx context.Context, workers int, handler Handler) {
	if b == nil || handler == nil {
		return
	}
	if workers < 1 {
		workers = 1
	}

	var wg sync.WaitGroup
	for i := 1; i <= workers; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer wg.Done()
			b.runWorkerLoop(ctx, workerID, handler)
		}()
	}

	go func() {
		<-ctx.Done()
		wg.Wait()
		b.cfg.Logger.Info("All AMQP consumer goroutines exited cleanly",
			slog.String("queue", b.cfg.QueueName),
		)
	}()
}

func (b *AMQPBroker) runWorkerLoop(ctx context.Context, workerID int, handler Handler) {
	backoff := b.cfg.ReconnectMin

	for {
		if ctx.Err() != nil {
			return
		}

		connected, err := b.runWorkerSession(ctx, workerID, handler)
		if ctx.Err() != nil || errors.Is(err, ErrBrokerClosed) {
			return
		}
		if connected {
			backoff = b.cfg.ReconnectMin
		}

		b.cfg.Logger.Warn("AMQP consumer disconnected; scheduling reconnect",
			slog.String("queue", b.cfg.QueueName),
			slog.Int("worker_id", workerID),
			slog.Duration("backoff", backoff),
			slog.Any("error", err),
		)

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		backoff *= 2
		if backoff > b.cfg.ReconnectMax {
			backoff = b.cfg.ReconnectMax
		}
	}
}

func (b *AMQPBroker) runWorkerSession(ctx context.Context, workerID int, handler Handler) (bool, error) {
	conn, err := b.ensureConnection()
	if err != nil {
		return false, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return false, fmt.Errorf("open consumer channel: %w", err)
	}
	defer ch.Close()

	if err := declareTopology(ch, b.cfg); err != nil {
		return false, err
	}

	// Enable publisher confirms on the consumer channel so retry and DLQ publishes
	// are durably confirmed before the original delivery is acknowledged.
	if err := ch.Confirm(false); err != nil {
		return false, fmt.Errorf("enable consumer channel confirms: %w", err)
	}

	if err := ch.Qos(b.cfg.Prefetch, 0, false); err != nil {
		return false, fmt.Errorf("set consumer qos: %w", err)
	}

	consumerTag := fmt.Sprintf("%s-%d", b.cfg.ConsumerPrefix, workerID)
	msgs, err := ch.ConsumeWithContext(
		ctx,
		b.cfg.QueueName,
		consumerTag,
		false, // auto-ack disabled
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return false, fmt.Errorf("register consumer: %w", err)
	}

	b.cfg.Logger.Info("AMQP consumer ready",
		slog.String("queue", b.cfg.QueueName),
		slog.Int("worker_id", workerID),
	)

	for {
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		case delivery, ok := <-msgs:
			if !ok {
				return true, errors.New("delivery channel closed by broker")
			}
			b.handleDelivery(ctx, ch, delivery, handler)
		}
	}
}

func (b *AMQPBroker) handleDelivery(ctx context.Context, ch *amqp.Channel, delivery amqp.Delivery, handler Handler) {
	headers := map[string]any(delivery.Headers)
	attempt := ExtractRetryCount(headers, b.cfg.RetryHeader)

	msg := Delivery{
		Body:      delivery.Body,
		Headers:   cloneHeaders(headers),
		Timestamp: delivery.Timestamp,
		Attempt:   attempt,
	}

	err := handler(ctx, msg)
	if err == nil {
		_ = delivery.Ack(false)
		return
	}

	permanent := IsPermanent(err)
	if permanent || attempt >= b.cfg.MaxRetries {
		if b.cfg.DLQName != "" {
			b.cfg.Logger.Error("Routing failed message to DLQ",
				slog.String("queue", b.cfg.QueueName),
				slog.String("dlq", b.cfg.DLQName),
				slog.Int("attempt", attempt),
				slog.Bool("permanent", permanent),
				slog.Any("error", err),
			)
			if dlqErr := b.publishConfirmed(ch, b.cfg.DLQName, delivery.Body, delivery.Headers, attempt, err, time.Now()); dlqErr != nil {
				// Never silently ack/discard a message if the DLQ publish was not durably confirmed.
				b.cfg.Logger.Error("DLQ publish failed; retaining message in primary queue",
					slog.String("queue", b.cfg.QueueName),
					slog.Any("dlq_error", dlqErr),
				)
				sleepOrCancel(ctx, 50*time.Millisecond)
				_ = delivery.Nack(false, true)
				return
			}
			b.invokeTerminalFailure(msg, err, permanent)
			_ = delivery.Ack(false)
			return
		}

		b.cfg.Logger.Warn("Dropping message after terminal failure",
			slog.String("queue", b.cfg.QueueName),
			slog.Int("attempt", attempt),
			slog.Bool("permanent", permanent),
			slog.Any("error", err),
		)
		b.invokeTerminalFailure(msg, err, permanent)
		_ = delivery.Nack(false, false)
		return
	}

	nextAttempt := attempt + 1
	b.cfg.Logger.Warn("Transient consumer failure; scheduling bounded retry",
		slog.String("queue", b.cfg.QueueName),
		slog.Int("attempt", nextAttempt),
		slog.Int("max_retries", b.cfg.MaxRetries),
		slog.Any("error", err),
	)

	ts := delivery.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	if pubErr := b.publishConfirmed(ch, b.cfg.QueueName, delivery.Body, delivery.Headers, nextAttempt, nil, ts); pubErr != nil {
		b.cfg.Logger.Error("Failed to republish message for retry; requeueing as-is",
			slog.String("queue", b.cfg.QueueName),
			slog.Any("error", pubErr),
		)
		sleepOrCancel(ctx, 50*time.Millisecond)
		_ = delivery.Nack(false, true)
		return
	}
	_ = delivery.Ack(false)
}

func (b *AMQPBroker) invokeTerminalFailure(msg Delivery, err error, permanent bool) {
	if b.cfg.OnTerminalFailure == nil {
		return
	}
	cbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b.cfg.OnTerminalFailure(cbCtx, msg, err, permanent)
}

func (b *AMQPBroker) publishConfirmed(
	ch *amqp.Channel,
	routingKey string,
	body []byte,
	origHeaders amqp.Table,
	retryCount int,
	cause error,
	ts time.Time,
) error {
	if ch == nil || routingKey == "" {
		if cause != nil {
			return ErrDLQUnavailable
		}
		return ErrPublishUnconfirm
	}

	headers := amqp.Table{}
	for k, v := range origHeaders {
		headers[k] = v
	}
	headers[b.cfg.RetryHeader] = int32(retryCount)
	if cause != nil {
		headers[DefaultErrorHeader] = cause.Error()
	}

	pubCtx, cancel := context.WithTimeout(context.Background(), b.cfg.PublishTimeout)
	defer cancel()

	confirm, err := ch.PublishWithDeferredConfirmWithContext(
		pubCtx,
		"",
		routingKey,
		true,  // mandatory
		false, // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Timestamp:    ts,
			Headers:      headers,
			Body:         body,
		},
	)
	if err != nil {
		return err
	}
	if confirm != nil {
		ack, waitErr := confirm.WaitContext(pubCtx)
		if waitErr != nil {
			return waitErr
		}
		if !ack {
			if cause != nil {
				return ErrDLQUnavailable
			}
			return ErrPublishUnconfirm
		}
	}
	return nil
}

// IsHealthy returns true if the underlying AMQP connection is open.
func (b *AMQPBroker) IsHealthy() bool {
	if b == nil {
		return false
	}
	b.connMu.RLock()
	defer b.connMu.RUnlock()
	return !b.closed && b.conn != nil && !b.conn.IsClosed()
}

// Close closes the publish channel and underlying AMQP connection.
func (b *AMQPBroker) Close() error {
	if b == nil {
		return nil
	}

	b.pubMu.Lock()
	pubCh := b.pubCh
	b.pubCh = nil
	b.pubMu.Unlock()

	b.connMu.Lock()
	b.closed = true
	conn := b.conn
	b.conn = nil
	b.connMu.Unlock()

	var errs []error
	if pubCh != nil && !pubCh.IsClosed() {
		if err := pubCh.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if conn != nil && !conn.IsClosed() {
		if err := conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close amqp broker (%s): %v", b.cfg.QueueName, errs)
	}
	return nil
}

func sleepOrCancel(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
