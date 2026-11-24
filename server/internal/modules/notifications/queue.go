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
	"sync"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"github.com/marknefedov/go-webpush"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	DefaultPushQueue   = "cadence_push_notifications"
	maxPushRetries     = 3
	pushRetryHeaderKey = "x-retry-count"
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
	conn      *amqp.Connection
	pubCh     *amqp.Channel
	pubMu     sync.Mutex
	repo      *Repository
	cfg       *config.Config
	logger    *slog.Logger
	vapidKeys *webpush.VAPIDKeys
	queueName string
}

func NewQueue(
	cfg *config.Config,
	repo *Repository,
	logger *slog.Logger,
) (*Queue, error) {
	if repo == nil {
		return nil, ErrNilRepository
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

	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("connect to rabbitmq for push queue: %w", err)
	}

	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open push rabbitmq channel: %w", err)
	}

	// Declare durable queue
	_, err = pubCh.QueueDeclare(
		DefaultPushQueue,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		_ = pubCh.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare rabbitmq push queue: %w", err)
	}

	return &Queue{
		conn:      conn,
		pubCh:     pubCh,
		repo:      repo,
		cfg:       cfg,
		logger:    logger,
		vapidKeys: vapidKeys,
		queueName: DefaultPushQueue,
	}, nil
}

// VAPIDPublicKey returns the active public key string.
func (q *Queue) VAPIDPublicKey() string {
	if q.vapidKeys != nil {
		return q.vapidKeys.PublicKeyString()
	}
	return q.cfg.VAPIDPublicKey
}

// ensurePublishChannelLocked reopens the AMQP connection or publish channel if closed. Caller must hold q.pubMu.
func (q *Queue) ensurePublishChannelLocked() error {
	if q.conn == nil || q.conn.IsClosed() {
		if q.cfg == nil || q.cfg.RabbitMQURL == "" {
			return errors.New("rabbitmq connection closed")
		}
		conn, err := amqp.Dial(q.cfg.RabbitMQURL)
		if err != nil {
			return fmt.Errorf("reconnect to rabbitmq for push queue: %w", err)
		}
		q.conn = conn
		q.pubCh = nil
	}
	if q.pubCh == nil || q.pubCh.IsClosed() {
		ch, err := q.conn.Channel()
		if err != nil {
			return fmt.Errorf("reopen push rabbitmq channel: %w", err)
		}
		_, _ = ch.QueueDeclare(q.queueName, true, false, false, false, nil)
		q.pubCh = ch
	}
	return nil
}

// Enqueue publishes a Web Push task to RabbitMQ safely under pubMu.
func (q *Queue) Enqueue(ctx context.Context, job PushJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal push job: %w", err)
	}

	q.pubMu.Lock()
	defer q.pubMu.Unlock()

	if err := q.ensurePublishChannelLocked(); err != nil {
		return err
	}

	err = q.pubCh.PublishWithContext(
		ctx,
		"",
		q.queueName,
		true,  // mandatory
		false, // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Timestamp:    time.Now(),
			Body:         payload,
		},
	)
	if err != nil {
		return fmt.Errorf("publish push job to rabbitmq: %w", err)
	}

	return nil
}

// StartConsumers spins up a pool of background worker goroutines to dispatch Web Push HTTP payloads.
func (q *Queue) StartConsumers(ctx context.Context, workers int) {
	var wg sync.WaitGroup

	for i := 1; i <= workers; i++ {
		wg.Add(1)
		workerID := i

		go func() {
			defer wg.Done()

			ch, err := q.conn.Channel()
			if err != nil {
				q.logger.Error("Failed to open rabbitmq channel for push worker", slog.Int("worker", workerID), slog.Any("error", err))
				return
			}
			defer ch.Close()

			if err := ch.Qos(10, 0, false); err != nil {
				q.logger.Error("Failed to set QoS on push channel", slog.Int("worker", workerID), slog.Any("error", err))
				return
			}

			msgs, err := ch.ConsumeWithContext(
				ctx,
				q.queueName,
				fmt.Sprintf("push-worker-%d", workerID),
				false, // auto-ack disabled
				false,
				false,
				false,
				nil,
			)
			if err != nil {
				q.logger.Error("Failed to register consumer on push queue", slog.Int("worker", workerID), slog.Any("error", err))
				return
			}

			q.logger.Info("Web Push worker ready", slog.Int("worker", workerID))

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

			for {
				select {
				case <-ctx.Done():
					q.logger.Info("Web Push worker shutting down", slog.Int("worker", workerID))
					return
				case delivery, ok := <-msgs:
					if !ok {
						return
					}

					var job PushJob
					if err := json.Unmarshal(delivery.Body, &job); err != nil {
						q.logger.Error("Malformed push message", slog.Any("error", err))
						_ = delivery.Nack(false, false)
						continue
					}

					keys, err := webpush.DecodeSubscriptionKeys(job.Auth, job.P256dh)
					if err != nil {
						q.logger.Warn("Invalid subscription keys; discarding", slog.Any("error", err))
						_ = delivery.Nack(false, false)
						continue
					}

					sub := webpush.Subscription{
						Endpoint: job.Endpoint,
						Keys:     keys,
					}

					sendCtx, sendCancel := context.WithTimeout(context.Background(), 10*time.Second)
					resp, err := webpush.SendNotification(sendCtx, job.Payload, &sub, options)
					sendCancel()

					if err != nil {
						q.logger.Warn("Failed to send webpush notification", slog.Any("error", err), slog.String("endpoint", job.Endpoint))
						q.retryOrDiscard(ch, delivery)
						continue
					}

					// Close response body
					if resp != nil && resp.Body != nil {
						_ = resp.Body.Close()
					}

					// Check gateway HTTP status code
					if resp != nil {
						if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
							// Subscription is no longer valid; auto-prune from database
							q.logger.Info("Auto-pruning expired push subscription",
								slog.Int("status", resp.StatusCode),
								slog.String("endpoint", job.Endpoint),
							)
							pruneCtx, pruneCancel := context.WithTimeout(context.Background(), 5*time.Second)
							_ = q.repo.DeleteSubscription(pruneCtx, job.Endpoint)
							pruneCancel()
						} else if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
							q.logger.Warn("Web Push gateway returned transient error; scheduling retry", slog.Int("status", resp.StatusCode))
							q.retryOrDiscard(ch, delivery)
							continue
						} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
							// Delivered successfully
						} else {
							q.logger.Warn("Web Push gateway returned unexpected status", slog.Int("status", resp.StatusCode))
						}
					}

					_ = delivery.Ack(false)
				}
			}
		}()
	}

	go func() {
		<-ctx.Done()
		wg.Wait()
		q.logger.Info("All Web Push workers exited cleanly")
	}()
}

func (q *Queue) retryOrDiscard(ch *amqp.Channel, delivery amqp.Delivery) {
	attempts := 0
	if v, ok := delivery.Headers[pushRetryHeaderKey]; ok {
		switch n := v.(type) {
		case int32:
			attempts = int(n)
		case int64:
			attempts = int(n)
		case int:
			attempts = n
		}
	}
	if attempts >= maxPushRetries {
		_ = delivery.Nack(false, false)
		return
	}
	headers := amqp.Table{}
	for k, v := range delivery.Headers {
		headers[k] = v
	}
	headers[pushRetryHeaderKey] = int32(attempts + 1)

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
		_ = delivery.Nack(false, true)
		return
	}
	_ = delivery.Ack(false)
}

// IsHealthy returns true if the RabbitMQ connection is active and not closed.
func (q *Queue) IsHealthy() bool {
	return q != nil && q.conn != nil && !q.conn.IsClosed()
}

// Close gracefully closes open channels and connection.
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
		return fmt.Errorf("close push queue: %v", errs)
	}
	return nil
}
