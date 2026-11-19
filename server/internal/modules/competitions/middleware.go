package competitions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

type compRawContextKey string

const competitionRawKey compRawContextKey = "competition_raw"

// WithCompetition stores the resolved Competition entity in context.
func WithCompetition(ctx context.Context, comp *Competition) context.Context {
	return context.WithValue(ctx, competitionRawKey, comp)
}

// GetCompetition retrieves the resolved Competition entity from context.
func GetCompetition(ctx context.Context) (*Competition, bool) {
	c, ok := ctx.Value(competitionRawKey).(*Competition)
	return c, ok
}

// ResolveCompetition middleware extracts the competitionSlug path parameter,
// verifies the competition exists (using Redis cache with DB fallback), and binds it to context.
func ResolveCompetition(repo *Repository, rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slug := chi.URLParam(r, "competitionSlug")
			if slug == "" {
				utils.ErrorJSON(w, http.StatusBadRequest, "competition slug parameter is required")
				return
			}

			cacheKey := "comp:slug:" + slug
			var comp Competition

			// 1. Check Redis cache (<1ms)
			if rdb != nil {
				if cached, err := rdb.Get(r.Context(), cacheKey).Bytes(); err == nil && len(cached) > 0 {
					if err := json.Unmarshal(cached, &comp); err == nil {
						ctx := WithCompetition(r.Context(), &comp)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}

			// 2. Query Postgres
			dbComp, err := repo.GetCompetitionBySlug(r.Context(), slug)
			if err != nil {
				if errors.Is(err, ErrCompetitionNotFound) {
					utils.ErrorJSON(w, http.StatusNotFound, fmt.Sprintf("competition '%s' not found", slug))
					return
				}
				utils.ErrorJSON(w, http.StatusInternalServerError, "failed to resolve competition")
				return
			}

			// 3. Cache in Redis (1h TTL)
			if rdb != nil {
				if data, err := json.Marshal(dbComp); err == nil {
					_ = rdb.Set(r.Context(), cacheKey, data, time.Hour)
				}
			}

			ctx := WithCompetition(r.Context(), dbComp)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireMembership middleware verifies that the authenticated user is an active member
// of the current competition, caches their role in Redis, and attaches CompetitionContext.
func RequireMembership(repo *Repository, rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.GetUserClaims(r.Context())
			if !ok || claims.UserID == "" {
				utils.ErrorJSON(w, http.StatusUnauthorized, "authentication required")
				return
			}

			comp, ok := GetCompetition(r.Context())
			if !ok || comp.ID == "" {
				utils.ErrorJSON(w, http.StatusInternalServerError, "competition context missing")
				return
			}

			memberKey := fmt.Sprintf("comp:%s:member:%s", comp.ID, claims.UserID)
			var role string

			// 1. Check Redis cache (<1ms)
			if rdb != nil {
				if cachedRole, err := rdb.Get(r.Context(), memberKey).Result(); err == nil && cachedRole != "" {
					role = cachedRole
				}
			}

			// 2. Query Postgres on cache miss
			if role == "" {
				dbRole, err := repo.GetMemberRole(r.Context(), comp.ID, claims.UserID)
				if err != nil {
					if errors.Is(err, ErrMemberNotFound) {
						utils.ErrorJSON(w, http.StatusForbidden, "you are not a member of this competition")
						return
					}
					utils.ErrorJSON(w, http.StatusInternalServerError, "failed to verify competition membership")
					return
				}
				role = dbRole

				// Cache in Redis (15m TTL)
				if rdb != nil {
					_ = rdb.Set(r.Context(), memberKey, role, 15*time.Minute)
				}
			}

			compCtx := &Context{
				CompetitionID: comp.ID,
				Slug:          comp.Slug,
				Name:          comp.Name,
				UserID:        claims.UserID,
				Role:          role,
			}

			ctx := WithCompetitionContext(r.Context(), compCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireCompetitionRoles verifies that the user's role in the active competition matches
// one of the allowed roles.
func RequireCompetitionRoles(allowedRoles ...string) func(http.Handler) http.Handler {
	roleSet := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		roleSet[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			compCtx, ok := FromContext(r.Context())
			if !ok {
				utils.ErrorJSON(w, http.StatusForbidden, "competition access context missing")
				return
			}

			_, allowed := roleSet[compCtx.Role]
			if !allowed && compCtx.Role == "owner" {
				_, allowed = roleSet["admin"]
			}
			if !allowed {
				utils.ErrorJSON(w, http.StatusForbidden, fmt.Sprintf("insufficient competition permissions; required one of %v", allowedRoles))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
