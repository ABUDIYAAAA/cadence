package ratelimit

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Result represents the evaluation of a rate limit check.
type Result struct {
	Allowed    bool
	Remaining  int64
	ResetAfter time.Duration
}

// Limiter provides sliding-window rate limiting backed by Redis and implemented entirely in Go.
type Limiter struct {
	redis *redis.Client
}

// NewLimiter creates a new Redis-backed Limiter.
func NewLimiter(client *redis.Client) *Limiter {
	return &Limiter{redis: client}
}

// Allow evaluates if a request with key is allowed within the limit and window using pure Go logic.
func (l *Limiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (*Result, error) {
	now := time.Now()
	nowMicro := float64(now.UnixMicro())
	clearBefore := float64(now.Add(-window).UnixMicro())

	// Step 1: Evict expired timestamps and fetch current count + oldest entry
	pipe := l.redis.TxPipeline()
	pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatFloat(clearBefore, 'f', -1, 64))
	countCmd := pipe.ZCard(ctx, key)
	oldestCmd := pipe.ZRangeWithScores(ctx, key, 0, 0)

	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("execute sliding window query: %w", err)
	}

	count := countCmd.Val()

	// Step 2: If under quota, record hit and refresh TTL
	if count < limit {
		member := strconv.FormatInt(now.UnixNano(), 10)
		addPipe := l.redis.TxPipeline()
		addPipe.ZAdd(ctx, key, redis.Z{Score: nowMicro, Member: member})
		addPipe.Expire(ctx, key, window)

		if _, err := addPipe.Exec(ctx); err != nil {
			return nil, fmt.Errorf("record rate limit hit: %w", err)
		}

		return &Result{
			Allowed:    true,
			Remaining:  limit - count - 1,
			ResetAfter: 0,
		}, nil
	}

	// Step 3: If limit reached, calculate when the oldest request slides out of the window
	resetAfter := window
	oldestList := oldestCmd.Val()
	if len(oldestList) > 0 {
		oldestMicro := oldestList[0].Score
		diffMicro := (oldestMicro + float64(window.Microseconds())) - nowMicro
		diffSec := math.Ceil(diffMicro / 1_000_000)
		if diffSec < 1 {
			diffSec = 1
		}
		resetAfter = time.Duration(diffSec) * time.Second
	}

	return &Result{
		Allowed:    false,
		Remaining:  0,
		ResetAfter: resetAfter,
	}, nil
}
