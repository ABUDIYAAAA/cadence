package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient initializes and returns a Redis client.
// The client manages an underlying connection pool and reconnects automatically.
func NewRedisClient(ctx context.Context, connString string) (*redis.Client, error) {
	opts, err := redis.ParseURL(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis connection string: %w", err)
	}

	opts.PoolSize = 25
	opts.MinIdleConns = 5
	opts.ConnMaxLifetime = 1 * time.Hour
	opts.ConnMaxIdleTime = 30 * time.Minute

	client := redis.NewClient(opts)

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return client, nil
}
