package competitions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/internal/modules/events"
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

// ResolveCompetition middleware extracts the competitionSlug path parameter within an event context,
// verifies the competition exists in that event (using Redis cache with DB fallback), and binds both
// Competition and CompetitionContext (inheriting the caller's validated role from EventContext).
func ResolveCompetition(repo *Repository, rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slug := chi.URLParam(r, "competitionSlug")
			if slug == "" {
				utils.ErrorJSON(w, http.StatusBadRequest, "competition slug parameter is required")
				return
			}

			var eventID string
			var userID, role string
			if evCtx, ok := events.FromContext(r.Context()); ok && evCtx != nil {
				eventID = evCtx.EventID
				userID = evCtx.UserID
				role = evCtx.Role
			} else if ev, ok := events.GetEvent(r.Context()); ok && ev != nil {
				eventID = ev.ID
			}
			if claims, ok := auth.GetUserClaims(r.Context()); ok && claims != nil {
				if userID == "" {
					userID = claims.UserID
				}
				if role == "" {
					if claims.Role == "dev" {
						role = "owner"
					} else {
						role = claims.Role
					}
				}
			}

			// If competition is already present in context matching the slug, ensure CompetitionContext is set and pass through
			if existing, ok := GetCompetition(r.Context()); ok && existing != nil && existing.Slug == slug && (eventID == "" || existing.EventID == eventID) {
				ctx := r.Context()
				if _, hasCompCtx := FromContext(ctx); !hasCompCtx && userID != "" {
					ctx = WithCompetitionContext(ctx, &Context{
						CompetitionID: existing.ID,
						EventID:       existing.EventID,
						Slug:          existing.Slug,
						Name:          existing.Name,
						UserID:        userID,
						Role:          role,
					})
				}
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if eventID == "" {
				eventID = events.DefaultEventID
			}

			cacheKey := fmt.Sprintf("event:%s:comp:slug:%s", eventID, slug)
			var comp *Competition

			// 1. Check Redis cache (<1ms)
			if rdb != nil {
				if cached, err := rdb.Get(r.Context(), cacheKey).Bytes(); err == nil && len(cached) > 0 {
					var c Competition
					if err := json.Unmarshal(cached, &c); err == nil {
						comp = &c
					}
				}
			}

			// 2. Query Postgres on cache miss
			if comp == nil {
				if repo == nil {
					utils.ErrorJSON(w, http.StatusInternalServerError, "failed to resolve competition")
					return
				}
				dbComp, err := repo.GetCompetitionBySlug(r.Context(), eventID, slug)
				if err != nil {
					if errors.Is(err, ErrCompetitionNotFound) {
						utils.ErrorJSON(w, http.StatusNotFound, fmt.Sprintf("competition '%s' not found in this event", slug))
						return
					}
					utils.ErrorJSON(w, http.StatusInternalServerError, "failed to resolve competition")
					return
				}
				comp = dbComp

				// 3. Cache in Redis (1h TTL)
				if rdb != nil {
					if data, err := json.Marshal(comp); err == nil {
						_ = rdb.Set(r.Context(), cacheKey, data, time.Hour)
					}
				}
			}

			ctx := WithCompetition(r.Context(), comp)
			if userID != "" {
				ctx = WithCompetitionContext(ctx, &Context{
					CompetitionID: comp.ID,
					EventID:       comp.EventID,
					Slug:          comp.Slug,
					Name:          comp.Name,
					UserID:        userID,
					Role:          role,
				})
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireCompetitionRoles verifies that the user's role in the active competition/event matches
// one of the allowed roles.
func RequireCompetitionRoles(allowedRoles ...string) func(http.Handler) http.Handler {
	roleSet := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		roleSet[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var activeRole string
			if compCtx, ok := FromContext(r.Context()); ok && compCtx != nil {
				activeRole = compCtx.Role
			} else if evCtx, ok := events.FromContext(r.Context()); ok && evCtx != nil {
				activeRole = evCtx.Role
			}

			if activeRole == "" {
				utils.ErrorJSON(w, http.StatusForbidden, "competition access context missing")
				return
			}

			// Dev role has top-level bypass across all competition roles
			if activeRole == "dev" {
				next.ServeHTTP(w, r)
				return
			}
			if claims, hasClaims := auth.GetUserClaims(r.Context()); hasClaims && claims.Role == "dev" {
				next.ServeHTTP(w, r)
				return
			}

			_, allowed := roleSet[activeRole]
			if !allowed && activeRole == "owner" {
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
