package ratelimit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/utils"
)

type contextKey string

const (
	DeviceIDCookieName            = "_did"
	DeviceIDContextKey contextKey = "device_id"

	DefaultUserLimit     int64 = 120
	DefaultDeviceLimit   int64 = 60
	DefaultCampusIPLimit int64 = 5000
)

// DeviceIDMiddleware ensures every incoming client possesses a cryptographically signed device_id cookie.
func DeviceIDMiddleware(cookieManager *utils.CookieManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deviceID, err := cookieManager.GetSignedCookie(r, DeviceIDCookieName)
			if err != nil || deviceID == "" {
				newID, genErr := utils.GenerateRandomToken(16)
				if genErr == nil {
					deviceID = newID
					// 1 year persistent device identifier
					cookieManager.SetSignedCookie(w, DeviceIDCookieName, deviceID, 365*24*time.Hour)
				}
			}

			ctx := context.WithValue(r.Context(), DeviceIDContextKey, deviceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetDeviceID retrieves the validated device identifier from the request context.
func GetDeviceID(ctx context.Context) string {
	if val, ok := ctx.Value(DeviceIDContextKey).(string); ok {
		return val
	}
	return "anonymous"
}

// RateLimitMiddleware enforces identity-based and device-based sliding-window rate limits backed by Redis.
func RateLimitMiddleware(
	limiter *Limiter,
	cfg *config.Config,
	cookieManager *utils.CookieManager,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			var key string
			var limit int64

			// 1. Try to resolve authenticated user identity from signed access_token cookie
			if tokenStr, err := cookieManager.GetSignedCookie(r, "access_token"); err == nil && tokenStr != "" {
				if claims, valErr := utils.ValidateAccessToken(tokenStr, cfg.JWTAccessSecret); valErr == nil {
					key = fmt.Sprintf("ratelimit:user:%s", claims.UserID)
					limit = DefaultUserLimit
				}
			}

			// 2. If unauthenticated, rate limit by signed device_id cookie (isolates devices behind NAT)
			if key == "" {
				deviceID := GetDeviceID(ctx)
				key = fmt.Sprintf("ratelimit:device:%s", deviceID)
				limit = DefaultDeviceLimit
			}

			// 3. Evaluate identity/device sliding window limit
			res, err := limiter.Allow(ctx, key, limit, 1*time.Minute)
			if err == nil {
				w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
				w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(res.Remaining, 10))
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(int64(res.ResetAfter.Seconds()), 10))

				if !res.Allowed {
					w.Header().Set("Retry-After", strconv.FormatInt(int64(res.ResetAfter.Seconds()), 10))
					utils.JSON(w, http.StatusTooManyRequests, map[string]any{
						"error":               "too many requests: rate limit exceeded",
						"retry_after_seconds": int(res.ResetAfter.Seconds()),
					})
					return
				}
			}

			// 4. Campus IP Global Ceiling safety net (prevents volumetric socket floods from overwhelming NAT)
			clientIP := extractRemoteIP(r.RemoteAddr)
			if clientIP != "" {
				ipKey := fmt.Sprintf("ratelimit:ip:%s", clientIP)
				ipRes, ipErr := limiter.Allow(ctx, ipKey, DefaultCampusIPLimit, 1*time.Minute)
				if ipErr == nil && !ipRes.Allowed {
					w.Header().Set("Retry-After", strconv.FormatInt(int64(ipRes.ResetAfter.Seconds()), 10))
					utils.JSON(w, http.StatusTooManyRequests, map[string]any{
						"error":               "campus gateway traffic ceiling exceeded; please retry shortly",
						"retry_after_seconds": int(ipRes.ResetAfter.Seconds()),
					})
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(remoteAddr)
}
