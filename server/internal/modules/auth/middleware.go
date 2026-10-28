package auth

import (
	"context"
	"net/http"
	"strings"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/pkg/utils"
)

type contextKey string

const userClaimsKey contextKey = "user_claims"

// AuthMiddleware creates an HTTP middleware that extracts and validates the access token from signed cookies.
func AuthMiddleware(cfg *config.Config, cookieManager *utils.CookieManager) func(http.Handler) http.Handler {
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

			if _, allowed := roleSet[claims.Role]; !allowed {
				utils.ErrorJSON(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetUserClaims retrieves token claims from request context.
func GetUserClaims(ctx context.Context) (*utils.AccessTokenClaims, bool) {
	claims, ok := ctx.Value(userClaimsKey).(*utils.AccessTokenClaims)
	return claims, ok
}
