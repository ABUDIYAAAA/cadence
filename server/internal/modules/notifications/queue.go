package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/amqpqueue"
	"github.com/marknefedov/go-webpush"
)

const (
	DefaultPushQueue   = "cadence_push_notifications"
	maxPushRetries     = 3
	pushRetryHeaderKey = amqpqueue.DefaultRetryHeader
)

var (
	ErrNilRepository = errors.New("repository cannot be nil")
)

type PushJob struct {
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
	Payload  []byte `json:"payload"`
}

type Queue struct {
	broker    amqpqueue.Broker
	repo      *Repository
	cfg       *config.Config
	logger    *slog.Logger
	vapidKeys *webpush.VAPIDKeys
}

func NewQueue(
	cfg *config.Config,
	repo *Repository,
	logger *slog.Logger,
) (*Queue, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	if logger == nil {
		logger = slog.Default()
	}

	// Parse VAPID keys: supports either direct base64 strings or a path to a .pem file
	var vapidKeys *webpush.VAPIDKeys
	if _, err := os.Stat(cfg.VAPIDPrivateKey); err == nil {
		pemBytes, err := os.ReadFile(cfg.VAPIDPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("read vapid private key pem file: %w", err)
		}
		loadedKeys, err := webpush.LoadVAPIDPrivateKeyPEM(pemBytes)
		if err != nil {
			return nil, fmt.Errorf("load vapid private key pem: %w", err)
		}
		vapidKeys = loadedKeys
	} else {
		rawVAPIDJSON := fmt.Sprintf(`{"publicKey":"%s","privateKey":"%s"}`, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey)
		var directKeys webpush.VAPIDKeys
		if err := json.Unmarshal([]byte(rawVAPIDJSON), &directKeys); err != nil {
			return nil, fmt.Errorf("parse vapid keys from config: %w", err)
		}
		vapidKeys = &directKeys
	}

	broker, err := amqpqueue.NewAMQPBroker(amqpqueue.Config{
		URL:            cfg.RabbitMQURL,
		QueueName:      DefaultPushQueue,
		ConsumerPrefix: "push-worker",
		Prefetch:       10,
		MaxRetries:     maxPushRetries,
		RetryHeader:    pushRetryHeaderKey,
		PublishTimeout: 5 * time.Second,
		Logger:         logger,
	})
	if err != nil {
		return nil, fmt.Errorf("init push amqp broker: %w", err)
	}

	return &Queue{
		broker:    broker,
		repo:      repo,
		cfg:       cfg,
		logger:    logger,
		vapidKeys: vapidKeys,
	}, nil
}

// NewQueueWithBroker builds a push Queue backed by any amqpqueue.Broker (e.g. MemoryBroker in tests).
func NewQueueWithBroker(
	broker amqpqueue.Broker,
	cfg *config.Config,
	repo *Repository,
	vapidKeys *webpush.VAPIDKeys,
	logger *slog.Logger,
) (*Queue, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Queue{
		broker:    broker,
		repo:      repo,
		cfg:       cfg,
		logger:    logger,
		vapidKeys: vapidKeys,
	}, nil
}

// VAPIDPublicKey returns the active public key string.
func (q *Queue) VAPIDPublicKey() string {
	if q.vapidKeys != nil {
		return q.vapidKeys.PublicKeyString()
	}
	return q.cfg.VAPIDPublicKey
}

// Enqueue publishes a Web Push task to RabbitMQ.
func (q *Queue) Enqueue(ctx context.Context, job PushJob) error {
	if q == nil || q.broker == nil {
		return amqpqueue.ErrBrokerClosed
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal push job: %w", err)
	}

	if err := q.broker.Publish(ctx, payload); err != nil {
		return fmt.Errorf("publish push job to rabbitmq: %w", err)
	}
	return nil
}

// StartConsumers spins up a pool of background worker goroutines to dispatch Web Push HTTP payloads.
func (q *Queue) StartConsumers(ctx context.Context, workers int) {
	if q == nil || q.broker == nil {
		return
	}

	safeTransport := &http.Transport{
		DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := (&net.Resolver{}).LookupIP(dialCtx, "ip", host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if IsRestrictedIP(ip) {
					return nil, fmt.Errorf("connection to restricted IP blocked: %s", ip.String())
				}
			}
			dialer := &net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}
			return dialer.DialContext(dialCtx, network, addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	options := &webpush.Options{
		Subscriber: q.cfg.VAPIDSubject,
		VAPIDKeys:  q.vapidKeys,
		TTL:        86400, // 24 hours
		HTTPClient: &http.Client{
			Transport: safeTransport,
			Timeout:   10 * time.Second,
		},
	}

	q.broker.StartConsumers(ctx, workers, func(_ context.Context, msg amqpqueue.Delivery) error {
		return q.handleDelivery(msg, options)
	})
}

func (q *Queue) handleDelivery(msg amqpqueue.Delivery, options *webpush.Options) error {
	var job PushJob
	if err := json.Unmarshal(msg.Body, &job); err != nil {
		return amqpqueue.Permanent(fmt.Errorf("malformed push message: %w", err))
	}

	keys, err := webpush.DecodeSubscriptionKeys(job.Auth, job.P256dh)
	if err != nil {
		return amqpqueue.Permanent(fmt.Errorf("invalid subscription keys: %w", err))
	}

	sub := webpush.Subscription{
		Endpoint: job.Endpoint,
		Keys:     keys,
	}

	sendCtx, sendCancel := context.WithTimeout(context.Background(), 10*time.Second)
	resp, err := webpush.SendNotification(sendCtx, job.Payload, &sub, options)
	sendCancel()

	if err != nil {
		q.logger.Warn("Failed to send webpush notification",
			slog.Any("error", err),
			slog.String("endpoint", job.Endpoint),
		)
		return err
	}

	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	if resp != nil {
		switch {
		case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
			// Subscription is no longer valid; auto-prune from database
			q.logger.Info("Auto-pruning expired push subscription",
				slog.Int("status", resp.StatusCode),
				slog.String("endpoint", job.Endpoint),
			)
			pruneCtx, pruneCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = q.repo.DeleteSubscription(pruneCtx, job.Endpoint)
			pruneCancel()
		case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
			q.logger.Warn("Web Push gateway returned transient error; scheduling retry",
				slog.Int("status", resp.StatusCode),
			)
			return fmt.Errorf("web push gateway transient status %d", resp.StatusCode)
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			// Delivered successfully
		default:
			q.logger.Warn("Web Push gateway returned unexpected status",
				slog.Int("status", resp.StatusCode),
			)
		}
	}

	return nil
}

// IsHealthy returns true if the RabbitMQ connection is active and not closed.
func (q *Queue) IsHealthy() bool {
	return q != nil && q.broker != nil && q.broker.IsHealthy()
}

// Close gracefully closes open channels and connection.
func (q *Queue) Close() error {
	if q == nil || q.broker == nil {
		return nil
	}
	if err := q.broker.Close(); err != nil && !errors.Is(err, amqpqueue.ErrBrokerClosed) {
		return fmt.Errorf("close push queue: %w", err)
	}
	return nil
}
