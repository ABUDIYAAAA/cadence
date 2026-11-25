package amqpqueue

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

const (
	DefaultRetryHeader    = "x-retry-count"
	DefaultErrorHeader    = "x-error"
	DefaultMaxRetries     = 3
	DefaultPrefetch       = 1
	DefaultPublishTimeout = 5 * time.Second
	DefaultReconnectMin   = 500 * time.Millisecond
	DefaultReconnectMax   = 15 * time.Second
)

var (
	ErrBrokerClosed     = errors.New("amqpqueue: broker is closed")
	ErrDLQUnavailable   = errors.New("amqpqueue: dead-letter queue unavailable or unconfirmed")
	ErrPublishUnconfirm = errors.New("amqpqueue: broker nacked or did not confirm message publish")
)

// permanentError marks a handler error as non-retryable so the broker routes it
// directly to the DLQ (or drops it) without consuming retry attempts.
type permanentError struct {
	err error
}

func (e *permanentError) Error() string {
	if e.err == nil {
		return "permanent error"
	}
	return e.err.Error()
}

func (e *permanentError) Unwrap() error {
	return e.err
}

// Permanent wraps err so consumers skip retries and route straight to DLQ/drop.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

// IsPermanent reports whether err (or any error in its chain) was marked permanent.
func IsPermanent(err error) bool {
	var pe *permanentError
	return errors.As(err, &pe)
}

// Delivery is a transport-agnostic message passed to queue handlers.
type Delivery struct {
	Body      []byte
	Headers   map[string]any
	Timestamp time.Time
	// Attempt is 0 on first delivery and increments on each bounded retry.
	Attempt int
}

// Handler processes a single delivery. Returning nil acknowledges the message;
// returning Permanent(err) routes to DLQ/drop immediately; any other error
// schedules a bounded retry up to Config.MaxRetries.
type Handler func(ctx context.Context, msg Delivery) error

// TerminalFailureFunc is invoked when a message is permanently dropped or moved
// to the DLQ (either due to a permanent error or exhausted retries).
type TerminalFailureFunc func(ctx context.Context, msg Delivery, err error, permanent bool)

// PublishOptions configures metadata on a published message.
type PublishOptions struct {
	ContentType string
	Timestamp   time.Time
	Headers     map[string]any
}

// PublishOption mutates PublishOptions.
type PublishOption func(*PublishOptions)

// WithTimestamp overrides the published message timestamp.
func WithTimestamp(ts time.Time) PublishOption {
	return func(o *PublishOptions) {
		o.Timestamp = ts
	}
}

// WithContentType overrides the published message content type (defaults to application/json).
func WithContentType(ct string) PublishOption {
	return func(o *PublishOptions) {
		o.ContentType = ct
	}
}

// WithHeaders attaches custom AMQP headers to a published message.
func WithHeaders(headers map[string]any) PublishOption {
	return func(o *PublishOptions) {
		if len(headers) == 0 {
			return
		}
		if o.Headers == nil {
			o.Headers = make(map[string]any, len(headers))
		}
		for k, v := range headers {
			o.Headers[k] = v
		}
	}
}

func resolvePublishOptions(opts []PublishOption) PublishOptions {
	resolved := PublishOptions{
		ContentType: "application/json",
		Timestamp:   time.Now(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&resolved)
		}
	}
	if resolved.ContentType == "" {
		resolved.ContentType = "application/json"
	}
	if resolved.Timestamp.IsZero() {
		resolved.Timestamp = time.Now()
	}
	return resolved
}

// Config controls queue topology, retry policy, QoS, and reconnection behavior.
type Config struct {
	URL               string
	QueueName         string
	DLQName           string
	ConsumerPrefix    string
	Prefetch          int
	MaxRetries        int
	RetryHeader       string
	PublishTimeout    time.Duration
	ReconnectMin      time.Duration
	ReconnectMax      time.Duration
	Logger            *slog.Logger
	OnTerminalFailure TerminalFailureFunc
}

func (c Config) withDefaults() Config {
	if c.ConsumerPrefix == "" {
		c.ConsumerPrefix = c.QueueName + "-worker"
	}
	if c.Prefetch <= 0 {
		c.Prefetch = DefaultPrefetch
	}
	if c.MaxRetries < 0 {
		c.MaxRetries = 0
	}
	if c.RetryHeader == "" {
		c.RetryHeader = DefaultRetryHeader
	}
	if c.PublishTimeout <= 0 {
		c.PublishTimeout = DefaultPublishTimeout
	}
	if c.ReconnectMin <= 0 {
		c.ReconnectMin = DefaultReconnectMin
	}
	if c.ReconnectMax <= 0 {
		c.ReconnectMax = DefaultReconnectMax
	}
	if c.ReconnectMax < c.ReconnectMin {
		c.ReconnectMax = c.ReconnectMin
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return c
}

// Broker is the port implemented by both the production AMQPBroker and the
// in-memory MemoryBroker used in unit tests.
type Broker interface {
	Publish(ctx context.Context, body []byte, opts ...PublishOption) error
	StartConsumers(ctx context.Context, workers int, handler Handler)
	IsHealthy() bool
	Close() error
}

// ExtractRetryCount reads an integer retry counter from AMQP/delivery headers.
func ExtractRetryCount(headers map[string]any, headerKey string) int {
	if headers == nil {
		return 0
	}
	if headerKey == "" {
		headerKey = DefaultRetryHeader
	}
	raw, ok := headers[headerKey]
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
	case uint:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	default:
		return 0
	}
}

func cloneHeaders(headers map[string]any) map[string]any {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]any, len(headers))
	for k, v := range headers {
		out[k] = v
	}
	return out
}
