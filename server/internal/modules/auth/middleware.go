package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/redis/go-redis/v9"
)

type contextKey string

const (
	userClaimsKey       contextKey = "user_claims"
	activeTenantRoleKey contextKey = "active_tenant_role"
)

// AuthMiddleware creates an HTTP middleware that extracts and validates the access token from signed cookies.
// It checks the instant Redis blocklist to ensure removed/deactivated accounts are locked out immediately (<1ms).
func AuthMiddleware(
	cfg *config.Config,
	cookieManager *utils.CookieManager,
	redisClient *redis.Client,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, err := cookieManager.GetSignedCookie(r, "access_token")
			if err != nil || tokenString == "" {
				// Fallback to Authorization header if present
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					tokenString = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if tokenString == "" {
				utils.ErrorJSON(w, http.StatusUnauthorized, "authentication required")
				return
			}

			claims, err := utils.ValidateAccessToken(tokenString, cfg.JWTAccessSecret)
			if err != nil {
				utils.ErrorJSON(w, http.StatusUnauthorized, "invalid or expired access token")
				return
			}

			// Check instant Redis blocklist and minimum token version
			if redisClient != nil {
				blockedKey := fmt.Sprintf("blocked:user:%s", claims.UserID)
				minVersionKey := fmt.Sprintf("user:min_token_version:%s", claims.UserID)

				pipe := redisClient.Pipeline()
				blockedCmd := pipe.Exists(r.Context(), blockedKey)
				minVersionCmd := pipe.Get(r.Context(), minVersionKey)
				_, pipeErr := pipe.Exec(r.Context())

				if pipeErr != nil && !errors.Is(pipeErr, redis.Nil) {
					// Fail closed on security verification if Redis is unreachable
					utils.ErrorJSON(w, http.StatusServiceUnavailable, "authentication service temporarily unavailable; please retry shortly")
					return
				}

				if blockedCmd.Val() > 0 {
					utils.ErrorJSON(w, http.StatusUnauthorized, "user account has been deactivated or removed")
					return
				}

				if minVersionStr, err := minVersionCmd.Result(); err == nil && minVersionStr != "" {
					var minVersion int
					if _, parseErr := fmt.Sscanf(minVersionStr, "%d", &minVersion); parseErr == nil {
						if claims.TokenVersion < minVersion {
							utils.ErrorJSON(w, http.StatusUnauthorized, "session revoked due to role change; please log in again")
							return
						}
					}
				}
			}

			ctx := context.WithValue(r.Context(), userClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRoles returns a middleware restricting route access to specified roles.
func RequireRoles(allowedRoles ...string) func(http.Handler) http.Handler {
	roleSet := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		roleSet[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetUserClaims(r.Context())
			if !ok {
				utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			// Dev role has top-level superuser bypass across all role requirements
			if claims.Role == "dev" {
				next.ServeHTTP(w, r)
				return
			}

			effectiveRole := GetEffectiveRole(r.Context())
			if effectiveRole == "dev" {
				next.ServeHTTP(w, r)
				return
			}

			_, allowed := roleSet[effectiveRole]
			if !allowed && effectiveRole == "owner" {
				_, allowed = roleSet["admin"]
			}
			if !allowed {
				utils.ErrorJSON(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireDev returns a middleware restricting route access exclusively to users with the 'dev' role.
func RequireDev() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetUserClaims(r.Context())
			if !ok {
				utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if claims.Role != "dev" {
				utils.ErrorJSON(w, http.StatusForbidden, "developer access required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WithUserClaims stores access token claims into context.
func WithUserClaims(ctx context.Context, claims *utils.AccessTokenClaims) context.Context {
	return context.WithValue(ctx, userClaimsKey, claims)
}

// GetUserClaims retrieves token claims from request context.
func GetUserClaims(ctx context.Context) (*utils.AccessTokenClaims, bool) {
	claims, ok := ctx.Value(userClaimsKey).(*utils.AccessTokenClaims)
	return claims, ok
}

// WithActiveTenantRole binds the caller's resolved role in the active Event/Competition scope
// without mutating the underlying JWT AccessTokenClaims.
func WithActiveTenantRole(ctx context.Context, role string) context.Context {
	if role == "" {
		return ctx
	}
	return context.WithValue(ctx, activeTenantRoleKey, role)
}

// GetEffectiveRole returns the caller's active Event/Competition role if resolved,
// falling back to their platform role in AccessTokenClaims.
func GetEffectiveRole(ctx context.Context) string {
	if role, ok := ctx.Value(activeTenantRoleKey).(string); ok && role != "" {
		return role
	}
	if claims, ok := GetUserClaims(ctx); ok && claims != nil {
		return claims.Role
	}
	return ""
}

// IsAdminRole reports whether role grants administrative privileges within a tenant scope.
func IsAdminRole(role string) bool {
	return role == "admin" || role == "owner" || role == "dev"
}
