package ratelimit

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// Result represents the evaluation of a rate limit check.
type Result struct {
	Allowed    bool
	Remaining  int64
	ResetAfter time.Duration
}

// slidingWindowScript atomically evicts expired entries, checks the current window count,
// inserts the new request hit when under quota, and returns {allowed, remaining, oldest_micro}.
// It uses redis.call("TIME") as the authoritative clock source to prevent multi-VPS NTP skew.
var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local member_prefix = ARGV[1]
local limit = tonumber(ARGV[2])
local window_ms = tonumber(ARGV[3])

-- 1. Obtain authoritative server time from Redis (sec, microsec)
local rtime = redis.call("TIME")
local now_sec = tonumber(rtime[1])
local now_usec = tonumber(rtime[2])
local now_micro = (now_sec * 1000000) + now_usec

local clear_before = now_micro - (window_ms * 1000)
local member = now_micro .. ":" .. member_prefix

-- 2. Evict expired entries older than the sliding window
redis.call("ZREMRANGEBYSCORE", key, "-inf", clear_before)

-- 3. Check current member count
local count = redis.call("ZCARD", key)

if count < limit then
	redis.call("ZADD", key, now_micro, member)
	redis.call("PEXPIRE", key, window_ms)
	return {1, limit - count - 1, "0", tostring(now_micro)}
end

-- 4. Over quota: retrieve oldest member score to calculate precise Retry-After
local oldest = redis.call("ZRANGE", key, 0, 0, "WITHSCORES")
local oldest_score = "0"
if oldest ~= nil and #oldest >= 2 then
	oldest_score = tostring(oldest[2])
end
return {0, 0, oldest_score, tostring(now_micro)}
`)

var memberCounter uint64

// Limiter provides atomic sliding-window rate limiting backed by Redis.
type Limiter struct {
	redis *redis.Client
}

// NewLimiter creates a new Redis-backed Limiter.
func NewLimiter(client *redis.Client) *Limiter {
	return &Limiter{redis: client}
}

// IsUserBlocked checks whether the user ID is present in the instant Redis blocklist.
func (l *Limiter) IsUserBlocked(ctx context.Context, userID string) bool {
	if l == nil || l.redis == nil || userID == "" {
		return false
	}
	blockedKey := fmt.Sprintf("blocked:user:%s", userID)
	exists, err := l.redis.Exists(ctx, blockedKey).Result()
	return err == nil && exists > 0
}

// Allow atomically evaluates if a request with key is allowed within the limit and window.
func (l *Limiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (*Result, error) {
	if l == nil || l.redis == nil {
		return &Result{Allowed: true, Remaining: limit}, nil
	}

	seq := atomic.AddUint64(&memberCounter, 1)
	windowMs := window.Milliseconds()
	if windowMs < 1 {
		windowMs = 1
	}

	raw, err := slidingWindowScript.Run(
		ctx,
		l.redis,
		[]string{key},
		seq,
		limit,
		windowMs,
	).Slice()
	if err != nil {
		return nil, fmt.Errorf("execute atomic sliding window script: %w", err)
	}
	if len(raw) < 4 {
		return nil, fmt.Errorf("unexpected sliding window script result length: %d", len(raw))
	}

	allowedInt, _ := raw[0].(int64)
	remainingInt, _ := raw[1].(int64)
	if allowedInt == 1 {
		return &Result{
			Allowed:    true,
			Remaining:  remainingInt,
			ResetAfter: 0,
		}, nil
	}

	resetAfter := window
	nowMicroStr, _ := raw[3].(string)
	nowMicro, _ := strconv.ParseFloat(nowMicroStr, 64)

	if oldestStr, ok := raw[2].(string); ok && oldestStr != "" && oldestStr != "0" {
		if oldestMicro, parseErr := strconv.ParseFloat(oldestStr, 64); parseErr == nil && oldestMicro > 0 && nowMicro > 0 {
			diffMicro := (oldestMicro + float64(window.Microseconds())) - nowMicro
			diffSec := math.Ceil(diffMicro / 1_000_000)
			if diffSec < 1 {
				diffSec = 1
			}
			resetAfter = time.Duration(diffSec) * time.Second
		}
	}

	return &Result{
		Allowed:    false,
		Remaining:  0,
		ResetAfter: resetAfter,
	}, nil
}
