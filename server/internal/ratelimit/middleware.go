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
	"devclubxnst.com/cadence/pkg/metrics"
	"devclubxnst.com/cadence/pkg/utils"
)

type contextKey string

const (
	DeviceIDCookieName                    = "_did"
	DeviceIDContextKey         contextKey = "device_id"
	DeviceIDVerifiedContextKey contextKey = "device_id_verified"

	DefaultGlobalLimit   int64 = 500
	DefaultUnauthLimit   int64 = 1000
	DefaultUserLimit     int64 = 500
	DefaultDeviceLimit   int64 = 500
	DefaultCampusIPLimit int64 = 5000
)

// DeviceIDMiddleware ensures every incoming client possesses a cryptographically signed device_id cookie.
func DeviceIDMiddleware(cookieManager *utils.CookieManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			verified := true
			deviceID, err := cookieManager.GetSignedCookie(r, DeviceIDCookieName)
			if err != nil || deviceID == "" {
				verified = false
				newID, genErr := utils.GenerateRandomToken(16)
				if genErr == nil {
					deviceID = newID
					// 1 year persistent device identifier
					cookieManager.SetSignedCookie(w, DeviceIDCookieName, deviceID, 365*24*time.Hour)
				}
			}

			ctx := context.WithValue(r.Context(), DeviceIDContextKey, deviceID)
			ctx = context.WithValue(ctx, DeviceIDVerifiedContextKey, verified)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetDeviceID retrieves the validated device identifier from the request context.
func GetDeviceID(ctx context.Context) string {
	if val, ok := ctx.Value(DeviceIDContextKey).(string); ok && val != "" {
		return val
	}
	return "anonymous"
}

// IsDeviceIDVerified reports whether the request arrived with an already-signed _did cookie.
func IsDeviceIDVerified(ctx context.Context) bool {
	val, _ := ctx.Value(DeviceIDVerifiedContextKey).(bool)
	return val
}

// RateLimitMiddleware enforces identity-based and device-based sliding-window rate limits backed by Redis.
func RateLimitMiddleware(
	limiter *Limiter,
	cfg *config.Config,
	cookieManager *utils.CookieManager,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Exempt CORS preflights, health checks, and authenticated Prometheus metrics scrapes.
			// Note: /metrics remains protected by metrics.AuthMiddleware; unauthenticated /metrics requests are not exempt.
			if r.Method == http.MethodOptions || r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			if r.URL.Path == "/metrics" && cfg != nil && metrics.HasValidScrapeToken(r, cfg.MetricsScrapeToken) {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			var key string
			limiterType := "device"

			globalLimit := DefaultGlobalLimit
			if cfg != nil && cfg.GlobalRateLimit > 0 {
				globalLimit = cfg.GlobalRateLimit
			}
			limit := globalLimit
			unauthLimit := DefaultUnauthLimit
			if cfg != nil && cfg.UnauthRateLimit > 0 {
				unauthLimit = cfg.UnauthRateLimit
			}

			// 1. Verify JWT before assigning a per-user bucket (signed access_token cookie or Bearer header)
			tokenStr := ""
			if cookieManager != nil {
				if signedToken, err := cookieManager.GetSignedCookie(r, "access_token"); err == nil && signedToken != "" {
					tokenStr = signedToken
				}
			}
			if tokenStr == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}
			if tokenStr != "" && cfg != nil && cfg.JWTAccessSecret != "" {
				if claims, valErr := utils.ValidateAccessToken(tokenStr, cfg.JWTAccessSecret); valErr == nil && claims != nil && claims.UserID != "" {
					if !limiter.IsUserBlocked(ctx, claims.UserID) {
						key = fmt.Sprintf("ratelimit:user:%s", claims.UserID)
						limit = globalLimit
						limiterType = "user"
					}
				}
			}

			// 2. For session refresh (/api/v1/auth/refresh) when access_token has expired:
			// require both a valid signed refresh_token cookie AND a verified signed _did cookie,
			// and enforce BOTH a refresh-token bucket and a stable refresh-session bucket (keyed by verified device_id)
			// so token rotation cannot bypass the limit while protecting session renewal from unauthenticated IP floods.
			if key == "" && r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/refresh" && cookieManager != nil {
				if refreshStr, err := cookieManager.GetSignedCookie(r, "refresh_token"); err == nil && refreshStr != "" && IsDeviceIDVerified(ctx) {
					deviceID := GetDeviceID(ctx)
					if deviceID != "" && deviceID != "anonymous" {
						sessionKey := fmt.Sprintf("ratelimit:refresh_session:%s", deviceID)
						sessionRes, sessionErr := limiter.Allow(ctx, sessionKey, globalLimit, 1*time.Minute)
						if sessionErr == nil {
							metrics.InstrumentRateLimit("refresh_session", sessionRes.Allowed)
							if !sessionRes.Allowed {
								writeRateLimitExceeded(w, globalLimit, sessionRes, "too many requests: rate limit exceeded")
								return
							}
						}

						key = fmt.Sprintf("ratelimit:refresh_token:%s", utils.HashToken(refreshStr))
						limit = globalLimit
						limiterType = "refresh"
					}
				}
			}

			// 3. If unauthenticated, rate limit by verified signed device_id cookie (isolates devices behind NAT).
			// When the client does NOT present a verified _did cookie (cookieless clients or cookie-dropping scripts),
			// key the unauthenticated limit on client IP so dropping cookies cannot reset the quota.
			if key == "" {
				if IsDeviceIDVerified(ctx) {
					deviceID := GetDeviceID(ctx)
					key = fmt.Sprintf("ratelimit:device:%s", deviceID)
					limit = unauthLimit
					limiterType = "device"
				} else {
					clientIP := extractRemoteIP(r.RemoteAddr)
					if clientIP == "" {
						clientIP = "unknown"
					}
					key = fmt.Sprintf("ratelimit:unauth_ip:%s", clientIP)
					limit = unauthLimit
					limiterType = "device"
				}
			}

			// 4. Evaluate primary sliding window limit (user, refresh_token, or device)
			res, err := limiter.Allow(ctx, key, limit, 1*time.Minute)
			if err == nil {
				w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
				w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(res.Remaining, 10))
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(int64(res.ResetAfter.Seconds()), 10))

				metrics.InstrumentRateLimit(limiterType, res.Allowed)

				if !res.Allowed {
					writeRateLimitExceeded(w, limit, res, "too many requests: rate limit exceeded")
					return
				}
			}

			// 5. Unauthenticated Campus IP Ceiling safety net (prevents volumetric unauthenticated socket floods
			// that drop _did cookies from overwhelming the server, without affecting authenticated users).
			if limiterType == "device" {
				clientIP := extractRemoteIP(r.RemoteAddr)
				if clientIP != "" {
					ipKey := fmt.Sprintf("ratelimit:ip:%s", clientIP)
					ipRes, ipErr := limiter.Allow(ctx, ipKey, DefaultCampusIPLimit, 1*time.Minute)
					if ipErr == nil {
						metrics.InstrumentRateLimit("campus_ip", ipRes.Allowed)
						if !ipRes.Allowed {
							w.Header().Set("Retry-After", strconv.FormatInt(int64(ipRes.ResetAfter.Seconds()), 10))
							utils.JSON(w, http.StatusTooManyRequests, map[string]any{
								"error":               "campus gateway traffic ceiling exceeded; please retry shortly",
								"retry_after_seconds": int(ipRes.ResetAfter.Seconds()),
							})
							return
						}
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeRateLimitExceeded(w http.ResponseWriter, limit int64, res *Result, msg string) {
	w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
	w.Header().Set("X-RateLimit-Remaining", "0")
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(int64(res.ResetAfter.Seconds()), 10))
	w.Header().Set("Retry-After", strconv.FormatInt(int64(res.ResetAfter.Seconds()), 10))
	utils.JSON(w, http.StatusTooManyRequests, map[string]any{
		"error":               msg,
		"retry_after_seconds": int(res.ResetAfter.Seconds()),
	})
}

func extractRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(remoteAddr)
}
