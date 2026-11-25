package amqpqueue

import (
	"context"
	"sync"
	"time"
)

// DeadLetter records a message routed to the DLQ (or dropped) in MemoryBroker.
type DeadLetter struct {
	Delivery  Delivery
	Err       error
	Permanent bool
}

// MemoryBroker is an in-memory Broker implementation for fast unit tests.
// It mirrors AMQPBroker's bounded retry, permanent error, DLQ, and
// OnTerminalFailure semantics without requiring a running RabbitMQ instance.
type MemoryBroker struct {
	cfg Config

	mu     sync.Mutex
	closed bool
	queue  chan Delivery
	dlq    []DeadLetter
	acked  []Delivery
}

// NewMemoryBroker creates an in-memory broker with the given Config.
func NewMemoryBroker(cfg Config) *MemoryBroker {
	cfg = cfg.withDefaults()
	return &MemoryBroker{
		cfg:   cfg,
		queue: make(chan Delivery, 256),
	}
}

// Publish enqueues a message in memory.
func (m *MemoryBroker) Publish(ctx context.Context, body []byte, opts ...PublishOption) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrBrokerClosed
	}
	m.mu.Unlock()

	po := resolvePublishOptions(opts)
	bodyCopy := append([]byte(nil), body...)
	headers := cloneHeaders(po.Headers)
	msg := Delivery{
		Body:      bodyCopy,
		Headers:   headers,
		Timestamp: po.Timestamp,
		Attempt:   ExtractRetryCount(headers, m.cfg.RetryHeader),
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case m.queue <- msg:
		return nil
	}
}

// StartConsumers launches worker goroutines draining the in-memory queue.
func (m *MemoryBroker) StartConsumers(ctx context.Context, workers int, handler Handler) {
	if m == nil || handler == nil {
		return
	}
	if workers < 1 {
		workers = 1
	}

	for i := 0; i < workers; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-m.queue:
					if !ok {
						return
					}
					m.handle(ctx, msg, handler)
				}
			}
		}()
	}
}

func (m *MemoryBroker) handle(ctx context.Context, msg Delivery, handler Handler) {
	err := handler(ctx, msg)
	if err == nil {
		m.mu.Lock()
		m.acked = append(m.acked, msg)
		m.mu.Unlock()
		return
	}

	permanent := IsPermanent(err)
	if permanent || msg.Attempt >= m.cfg.MaxRetries {
		if m.cfg.DLQName != "" {
			headers := cloneHeaders(msg.Headers)
			if headers == nil {
				headers = map[string]any{}
			}
			headers[m.cfg.RetryHeader] = int32(msg.Attempt)
			headers[DefaultErrorHeader] = err.Error()
			msg.Headers = headers
		}
		m.mu.Lock()
		m.dlq = append(m.dlq, DeadLetter{
			Delivery:  msg,
			Err:       err,
			Permanent: permanent,
		})
		m.mu.Unlock()

		if m.cfg.OnTerminalFailure != nil {
			cbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			m.cfg.OnTerminalFailure(cbCtx, msg, err, permanent)
			cancel()
		}
		return
	}

	nextAttempt := msg.Attempt + 1
	headers := cloneHeaders(msg.Headers)
	if headers == nil {
		headers = map[string]any{}
	}
	headers[m.cfg.RetryHeader] = int32(nextAttempt)

	retryMsg := Delivery{
		Body:      append([]byte(nil), msg.Body...),
		Headers:   headers,
		Timestamp: msg.Timestamp,
		Attempt:   nextAttempt,
	}

	select {
	case <-ctx.Done():
	case m.queue <- retryMsg:
	}
}

// DrainQueue non-blockingly drains all currently queued (unconsumed) deliveries.
func (m *MemoryBroker) DrainQueue() []Delivery {
	var out []Delivery
	for {
		select {
		case msg, ok := <-m.queue:
			if !ok {
				return out
			}
			out = append(out, msg)
		default:
			return out
		}
	}
}

// DeadLetters returns a snapshot of all messages routed to the DLQ or dropped.
func (m *MemoryBroker) DeadLetters() []DeadLetter {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]DeadLetter, len(m.dlq))
	copy(out, m.dlq)
	return out
}

// Acked returns a snapshot of all successfully acknowledged messages.
func (m *MemoryBroker) Acked() []Delivery {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Delivery, len(m.acked))
	copy(out, m.acked)
	return out
}

// IsHealthy returns true while the memory broker is open.
func (m *MemoryBroker) IsHealthy() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return !m.closed
}

// Close marks the memory broker closed and stops accepting publishes.
func (m *MemoryBroker) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.queue)
	}
	return nil
}
