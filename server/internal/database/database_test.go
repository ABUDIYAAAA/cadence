package database_test

import (
	"context"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/database"
)

func TestNewPool_FailsFastWhenUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Port 54321 on localhost should not have PostgreSQL listening
	unreachableURL := "postgres://cadence_user:cadence_pass@127.0.0.1:54321/cadence_nonexistent?sslmode=disable&connect_timeout=2"

	start := time.Now()
	pool, err := database.NewPool(ctx, unreachableURL)
	duration := time.Since(start)

	if err == nil {
		if pool != nil {
			pool.Close()
		}
		t.Fatal("expected NewPool to fail fast when PostgreSQL is unreachable, but got nil error")
	}

	// Ensure it didn't hang indefinitely and completed within reasonable timeout
	if duration > 7*time.Second {
		t.Fatalf("expected NewPool to fail fast within 7s, but took %v", duration)
	}
}

func TestNewRedisClient_FailsFastWhenUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Port 56379 on localhost should not have Redis listening
	unreachableURL := "redis://127.0.0.1:56379/0?dial_timeout=2s"

	start := time.Now()
	client, err := database.NewRedisClient(ctx, unreachableURL)
	duration := time.Since(start)

	if err == nil {
		if client != nil {
			_ = client.Close()
		}
		t.Fatal("expected NewRedisClient to fail fast when Redis is unreachable, but got nil error")
	}

	if duration > 7*time.Second {
		t.Fatalf("expected NewRedisClient to fail fast within 7s, but took %v", duration)
	}
}
