package amqpqueue

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryBroker_PublishAndAck(t *testing.T) {
	broker := NewMemoryBroker(Config{
		QueueName:  "test_queue",
		MaxRetries: 3,
	})
	defer broker.Close()

	if !broker.IsHealthy() {
		t.Fatal("expected healthy memory broker")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	received := make(chan Delivery, 1)
	broker.StartConsumers(ctx, 2, func(_ context.Context, msg Delivery) error {
		received <- msg
		return nil
	})

	ts := time.Now().Truncate(time.Second)
	if err := broker.Publish(ctx, []byte(`{"ok":true}`), WithTimestamp(ts), WithHeaders(map[string]any{"x-trace": "abc"})); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	select {
	case msg := <-received:
		if string(msg.Body) != `{"ok":true}` {
			t.Fatalf("unexpected body: %s", string(msg.Body))
		}
		if msg.Attempt != 0 {
			t.Fatalf("expected attempt 0, got %d", msg.Attempt)
		}
		if msg.Headers["x-trace"] != "abc" {
			t.Fatalf("expected header x-trace=abc, got %v", msg.Headers["x-trace"])
		}
		if !msg.Timestamp.Equal(ts) {
			t.Fatalf("expected timestamp %v, got %v", ts, msg.Timestamp)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for delivery")
	}

	waitUntil(t, 2*time.Second, func() bool { return len(broker.Acked()) == 1 })
	if len(broker.DeadLetters()) != 0 {
		t.Fatalf("expected 0 dead letters, got %d", len(broker.DeadLetters()))
	}
}

func TestMemoryBroker_TransientRetriesThenSucceeds(t *testing.T) {
	broker := NewMemoryBroker(Config{
		QueueName:  "test_retry",
		DLQName:    "test_retry_dlq",
		MaxRetries: 3,
	})
	defer broker.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var attempts atomic.Int32
	done := make(chan int, 1)

	broker.StartConsumers(ctx, 1, func(_ context.Context, msg Delivery) error {
		n := attempts.Add(1)
		if n < 3 {
			return errors.New("temporary db hiccup")
		}
		done <- msg.Attempt
		return nil
	})

	if err := broker.Publish(ctx, []byte(`"event"`)); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	select {
	case attempt := <-done:
		if attempt != 2 {
			t.Fatalf("expected success on attempt 2 (3rd delivery), got %d", attempt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for retry success")
	}

	waitUntil(t, 2*time.Second, func() bool { return len(broker.Acked()) == 1 })
	if len(broker.DeadLetters()) != 0 {
		t.Fatalf("expected 0 dead letters, got %d", len(broker.DeadLetters()))
	}
}

func TestMemoryBroker_ExhaustedRetriesRouteToDLQAndInvokeCallback(t *testing.T) {
	terminalCalled := make(chan bool, 1)
	broker := NewMemoryBroker(Config{
		QueueName:  "test_dlq",
		DLQName:    "test_dlq_dead",
		MaxRetries: 2,
		OnTerminalFailure: func(_ context.Context, msg Delivery, err error, permanent bool) {
			terminalCalled <- permanent
		},
	})
	defer broker.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var deliveries atomic.Int32
	broker.StartConsumers(ctx, 1, func(_ context.Context, _ Delivery) error {
		deliveries.Add(1)
		return errors.New("persistent transient failure")
	})

	if err := broker.Publish(ctx, []byte(`"poison"`)); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	select {
	case permanent := <-terminalCalled:
		if permanent {
			t.Fatal("expected permanent=false for exhausted retries")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for terminal failure callback")
	}

	// Attempt 0, 1, 2 = 3 total deliveries (initial + 2 retries).
	if got := deliveries.Load(); got != 3 {
		t.Fatalf("expected 3 deliveries, got %d", got)
	}

	dlq := broker.DeadLetters()
	if len(dlq) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(dlq))
	}
	if dlq[0].Delivery.Attempt != 2 {
		t.Fatalf("expected dead letter attempt=2, got %d", dlq[0].Delivery.Attempt)
	}
	if dlq[0].Delivery.Headers[DefaultErrorHeader] != "persistent transient failure" {
		t.Fatalf("unexpected x-error header: %v", dlq[0].Delivery.Headers[DefaultErrorHeader])
	}
}

func TestMemoryBroker_PermanentErrorSkipsRetries(t *testing.T) {
	terminalCalled := make(chan bool, 1)
	broker := NewMemoryBroker(Config{
		QueueName:  "test_perm",
		DLQName:    "test_perm_dlq",
		MaxRetries: 3,
		OnTerminalFailure: func(_ context.Context, _ Delivery, _ error, permanent bool) {
			terminalCalled <- permanent
		},
	})
	defer broker.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var deliveries atomic.Int32
	broker.StartConsumers(ctx, 1, func(_ context.Context, _ Delivery) error {
		deliveries.Add(1)
		return Permanent(errors.New("malformed json"))
	})

	if err := broker.Publish(ctx, []byte(`{bad`)); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	select {
	case permanent := <-terminalCalled:
		if !permanent {
			t.Fatal("expected permanent=true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for terminal failure callback")
	}

	if got := deliveries.Load(); got != 1 {
		t.Fatalf("expected exactly 1 delivery for permanent error, got %d", got)
	}
	dlq := broker.DeadLetters()
	if len(dlq) != 1 || !dlq[0].Permanent {
		t.Fatalf("expected 1 permanent dead letter, got %+v", dlq)
	}
}

func TestMemoryBroker_ClosePreventsPublish(t *testing.T) {
	broker := NewMemoryBroker(Config{QueueName: "test_close"})
	if err := broker.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if broker.IsHealthy() {
		t.Fatal("expected unhealthy after Close")
	}
	if err := broker.Publish(context.Background(), []byte("x")); !errors.Is(err, ErrBrokerClosed) {
		t.Fatalf("expected ErrBrokerClosed, got %v", err)
	}
}

func TestExtractRetryCount(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]any
		want    int
	}{
		{"nil", nil, 0},
		{"missing", map[string]any{}, 0},
		{"int", map[string]any{DefaultRetryHeader: 2}, 2},
		{"int32", map[string]any{DefaultRetryHeader: int32(3)}, 3},
		{"int64", map[string]any{DefaultRetryHeader: int64(4)}, 4},
		{"float64", map[string]any{DefaultRetryHeader: float64(5)}, 5},
		{"invalid_string", map[string]any{DefaultRetryHeader: "3"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractRetryCount(tc.headers, ""); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestPermanentErrorWrapping(t *testing.T) {
	base := errors.New("boom")
	wrapped := Permanent(base)
	if !IsPermanent(wrapped) {
		t.Fatal("expected IsPermanent to be true")
	}
	if !errors.Is(wrapped, base) {
		t.Fatal("expected errors.Is to unwrap to base error")
	}
	if IsPermanent(base) {
		t.Fatal("expected IsPermanent(base) to be false")
	}
	if Permanent(nil) != nil {
		t.Fatal("expected Permanent(nil) to return nil")
	}
}

func waitUntil(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
