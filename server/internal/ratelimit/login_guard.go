package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"devclubxnst.com/cadence/pkg/metrics"
	"github.com/redis/go-redis/v9"
)

var (
	ErrLoginGuardUnavailable = errors.New("login guard storage unavailable")
)

const (
	// DefaultMaxDeviceEmailLogins: hard throttle for single device against single email (5 / 5m).
	DefaultMaxDeviceEmailLogins = 5

	// Account lockout mitigation:
	// To prevent unauthenticated DoS (attacker locking out a user by spamming incorrect passwords),
	// after AccountWarnAttempts (5) we start progressive delay throttling, with max lockout at 20.
	DefaultAccountWarnAttempts = 5
	DefaultMaxAccountAttempts  = 20

	// Campus NAT spray protection:
	// A thousand people can sign in at once from one campus IP, and some will mistype, so the per-IP
	// cutoff is high (3000 failures per 5 minutes). Per-account and per-device limits still apply.
	DefaultMaxIPFailedLogins = 3000

	DefaultLoginBlockTTL = 5 * time.Minute
)

// LoginGuard provides multi-dimensional brute-force and credential spraying protection
// using standard Redis pipelined commands (no Lua scripting).
type LoginGuard struct {
	redis              *redis.Client
	maxDeviceEmail     int64
	maxAccountAttempts int64
	maxIPAttempts      int64
	blockDuration      time.Duration
}

// NewLoginGuard creates a new LoginGuard instance.
func NewLoginGuard(client *redis.Client) *LoginGuard {
	return &LoginGuard{
		redis:              client,
		maxDeviceEmail:     DefaultMaxDeviceEmailLogins,
		maxAccountAttempts: DefaultMaxAccountAttempts,
		maxIPAttempts:      DefaultMaxIPFailedLogins,
		blockDuration:      DefaultLoginBlockTTL,
	}
}

// WithMaxIPAttempts overrides the failed-login threshold per client IP (values <= 0 are ignored).
func (lg *LoginGuard) WithMaxIPAttempts(n int64) *LoginGuard {
	if n > 0 {
		lg.maxIPAttempts = n
	}
	return lg
}

type checkTarget struct {
	key       string
	limit     int64
	dimension string
}

// CheckAllowed verifies whether login attempts for the device, email, and client IP are permitted
// using standard pipelined MGet and TTL queries.
func (lg *LoginGuard) CheckAllowed(ctx context.Context, deviceID, email string, clientIP ...string) (bool, time.Duration, error) {
	if lg.redis == nil {
		return false, 0, ErrLoginGuardUnavailable
	}

	targets := make([]checkTarget, 0, 3)
	if strings.TrimSpace(deviceID) != "" {
		targets = append(targets, checkTarget{
			key:       lg.buildKey(deviceID, email),
			limit:     lg.maxDeviceEmail,
			dimension: "device_email",
		})
	}
	if strings.TrimSpace(email) != "" {
		targets = append(targets, checkTarget{
			key:       lg.buildEmailKey(email),
			limit:     lg.maxAccountAttempts,
			dimension: "account_email",
		})
	}
	if len(clientIP) > 0 && strings.TrimSpace(clientIP[0]) != "" {
		targets = append(targets, checkTarget{
			key:       lg.buildIPKey(clientIP[0]),
			limit:     lg.maxIPAttempts,
			dimension: "client_ip",
		})
	}

	if len(targets) == 0 {
		return true, 0, nil
	}

	// Read all target keys in a single round-trip pipeline
	pipe := lg.redis.Pipeline()
	getCmds := make([]*redis.StringCmd, len(targets))
	ttlCmds := make([]*redis.DurationCmd, len(targets))
	for i, t := range targets {
		getCmds[i] = pipe.Get(ctx, t.key)
		ttlCmds[i] = pipe.TTL(ctx, t.key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return false, 0, fmt.Errorf("check login guard: %w", err)
	}

	for i, t := range targets {
		val, err := getCmds[i].Int64()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			continue
		}

		if val >= t.limit {
			ttl, ttlErr := ttlCmds[i].Result()
			if ttlErr != nil || ttl <= 0 {
				ttl = lg.blockDuration
			}
			metrics.InstrumentRateLimit(t.dimension, false)
			return false, ttl, nil
		}
	}

	metrics.InstrumentRateLimit("login_guard", true)
	return true, 0, nil
}

// RecordFailure increments failed attempt counters using a standard Redis pipeline with INCR and EXPIRE.
func (lg *LoginGuard) RecordFailure(ctx context.Context, deviceID, email string, clientIP ...string) error {
	if lg.redis == nil {
		return ErrLoginGuardUnavailable
	}

	keys := make([]string, 0, 3)
	if strings.TrimSpace(deviceID) != "" {
		keys = append(keys, lg.buildKey(deviceID, email))
	}
	if strings.TrimSpace(email) != "" {
		keys = append(keys, lg.buildEmailKey(email))
	}
	if len(clientIP) > 0 && strings.TrimSpace(clientIP[0]) != "" {
		keys = append(keys, lg.buildIPKey(clientIP[0]))
	}

	if len(keys) == 0 {
		return nil
	}

	pipe := lg.redis.Pipeline()
	for _, key := range keys {
		pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, lg.blockDuration)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("record login failure: %w", err)
	}
	return nil
}

// ResetSuccess clears failed attempt records for (device + email) and the email account on successful authentication.
// The per-IP spray counter is deliberately preserved to prevent an attacker from clearing IP-wide spray counts.
func (lg *LoginGuard) ResetSuccess(ctx context.Context, deviceID, email string) error {
	if lg.redis == nil {
		return nil
	}
	keys := make([]string, 0, 2)
	if strings.TrimSpace(deviceID) != "" {
		keys = append(keys, lg.buildKey(deviceID, email))
	}
	if strings.TrimSpace(email) != "" {
		keys = append(keys, lg.buildEmailKey(email))
	}
	if len(keys) == 0 {
		return nil
	}
	return lg.redis.Del(ctx, keys...).Err()
}

func (lg *LoginGuard) buildKey(deviceID, email string) string {
	return fmt.Sprintf("ratelimit:login:device:%s:email:%s", deviceID, hashEmail(email))
}

func (lg *LoginGuard) buildEmailKey(email string) string {
	return fmt.Sprintf("ratelimit:login:email:%s", hashEmail(email))
}

func (lg *LoginGuard) buildIPKey(clientIP string) string {
	return fmt.Sprintf("ratelimit:login:ip:%s", strings.TrimSpace(clientIP))
}

func hashEmail(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}
