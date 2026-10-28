package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultMaxFailedLogins = 5
	DefaultLoginBlockTTL   = 5 * time.Minute
)

// LoginGuard provides protection against credential brute-forcing and account DoS attacks.
type LoginGuard struct {
	redis          *redis.Client
	maxAttempts    int64
	blockDuration  time.Duration
}

// NewLoginGuard creates a new LoginGuard instance.
func NewLoginGuard(client *redis.Client) *LoginGuard {
	return &LoginGuard{
		redis:         client,
		maxAttempts:   DefaultMaxFailedLogins,
		blockDuration: DefaultLoginBlockTTL,
	}
}

// CheckAllowed verifies whether login attempts for the device + email combination are permitted.
func (lg *LoginGuard) CheckAllowed(ctx context.Context, deviceID, email string) (bool, time.Duration, error) {
	key := lg.buildKey(deviceID, email)
	val, err := lg.redis.Get(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			return true, 0, nil
		}
		return false, 0, fmt.Errorf("check login guard: %w", err)
	}

	if val >= lg.maxAttempts {
		ttl, err := lg.redis.TTL(ctx, key).Result()
		if err != nil {
			ttl = lg.blockDuration
		}
		return false, ttl, nil
	}

	return true, 0, nil
}

// RecordFailure increments the failed attempt count for device + email.
func (lg *LoginGuard) RecordFailure(ctx context.Context, deviceID, email string) error {
	key := lg.buildKey(deviceID, email)
	pipe := lg.redis.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, lg.blockDuration)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("record login failure: %w", err)
	}

	_ = incr.Val()
	return nil
}

// ResetSuccess clears failed attempt records on successful authentication.
func (lg *LoginGuard) ResetSuccess(ctx context.Context, deviceID, email string) error {
	key := lg.buildKey(deviceID, email)
	return lg.redis.Del(ctx, key).Err()
}

func (lg *LoginGuard) buildKey(deviceID, email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	hash := sha256.Sum256([]byte(normalized))
	emailHash := hex.EncodeToString(hash[:])
	return fmt.Sprintf("ratelimit:login:device:%s:email:%s", deviceID, emailHash)
}
