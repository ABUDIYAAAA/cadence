package events

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

type eventRawContextKey string

const eventRawKey eventRawContextKey = "event_raw"

// WithEvent stores the resolved Event entity in context.
func WithEvent(ctx context.Context, ev *Event) context.Context {
	return context.WithValue(ctx, eventRawKey, ev)
}

// GetEvent retrieves the resolved Event entity from context.
func GetEvent(ctx context.Context) (*Event, bool) {
	e, ok := ctx.Value(eventRawKey).(*Event)
	return e, ok
}

// ResolveEvent middleware extracts the eventSlug path parameter,
// verifies the event exists (using Redis cache with DB fallback), and binds it to context.
func ResolveEvent(repo *Repository, rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slug := chi.URLParam(r, "eventSlug")
			if slug == "" {
				utils.ErrorJSON(w, http.StatusBadRequest, "event slug parameter is required")
				return
			}

			// If event is already present in context matching the slug, pass through
			if existing, ok := GetEvent(r.Context()); ok && existing != nil && existing.Slug == slug {
				next.ServeHTTP(w, r)
				return
			}

			cacheKey := "event:slug:" + slug
			var ev Event

			// 1. Check Redis cache (<1ms)
			if rdb != nil {
				if cached, err := rdb.Get(r.Context(), cacheKey).Bytes(); err == nil && len(cached) > 0 {
					if err := json.Unmarshal(cached, &ev); err == nil {
						ctx := WithEvent(r.Context(), &ev)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}

			// 2. Query Postgres
			if repo == nil {
				utils.ErrorJSON(w, http.StatusInternalServerError, "failed to resolve event")
				return
			}
			dbEv, err := repo.GetEventBySlug(r.Context(), slug)
			if err != nil {
				if errors.Is(err, ErrEventNotFound) {
					utils.ErrorJSON(w, http.StatusNotFound, fmt.Sprintf("event '%s' not found", slug))
					return
				}
				utils.ErrorJSON(w, http.StatusInternalServerError, "failed to resolve event")
				return
			}

			// 3. Cache in Redis (1h TTL)
			if rdb != nil {
				if data, err := json.Marshal(dbEv); err == nil {
					_ = rdb.Set(r.Context(), cacheKey, data, time.Hour)
				}
			}

			ctx := WithEvent(r.Context(), dbEv)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireMembership middleware verifies that the authenticated user is an active member
// of the current event, caches their role and accessible events in Redis, and attaches EventContext.
// Banned members are immediately rejected with 403 Forbidden (blocking access to the event and all related competitions).
func RequireMembership(repo *Repository, rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.GetUserClaims(r.Context())
			if !ok || claims.UserID == "" {
				utils.ErrorJSON(w, http.StatusUnauthorized, "authentication required")
				return
			}

			ev, ok := GetEvent(r.Context())
			if !ok || ev.ID == "" {
				utils.ErrorJSON(w, http.StatusInternalServerError, "event context missing")
				return
			}

			// Dev bypass: global dev automatically gets full owner powers across all events
			if claims.GlobalRole == "dev" {
				evCtx := &Context{
					EventID: ev.ID,
					Slug:    ev.Slug,
					Name:    ev.Name,
					UserID:  claims.UserID,
					Role:    "owner",
				}
				updatedClaims := *claims
				updatedClaims.Role = "owner"
				ctx := auth.WithUserClaims(r.Context(), &updatedClaims)
				ctx = WithEventContext(ctx, evCtx)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			memberKey := fmt.Sprintf("event:%s:member:%s", ev.ID, claims.UserID)
			var role string

			// 1. Check Redis cache (<1ms)
			if rdb != nil {
				if cachedRole, err := rdb.Get(r.Context(), memberKey).Result(); err == nil && cachedRole != "" {
					if cachedRole == "banned" {
						utils.ErrorJSON(w, http.StatusForbidden, "you are banned from this event")
						return
					}
					role = cachedRole
				}
			}

			// 2. Query Postgres on cache miss
			if role == "" {
				if repo == nil {
					utils.ErrorJSON(w, http.StatusInternalServerError, "failed to verify event membership")
					return
				}
				dbRole, isBanned, err := repo.GetMemberRole(r.Context(), ev.ID, claims.UserID)
				if err != nil {
					if errors.Is(err, ErrMemberNotFound) {
						utils.ErrorJSON(w, http.StatusForbidden, "you are not a member of this event")
						return
					}
					utils.ErrorJSON(w, http.StatusInternalServerError, "failed to verify event membership")
					return
				}

				if isBanned {
					if rdb != nil {
						_ = rdb.Set(r.Context(), memberKey, "banned", 15*time.Minute).Err()
						_ = rdb.SRem(r.Context(), fmt.Sprintf("user:%s:events", claims.UserID), ev.ID).Err()
					}
					utils.ErrorJSON(w, http.StatusForbidden, "you are banned from this event")
					return
				}

				role = dbRole

				// Cache in Redis (15m TTL) and update accessible events set (1h TTL)
				if rdb != nil {
					_ = rdb.Set(r.Context(), memberKey, role, 15*time.Minute).Err()
					userEventsKey := fmt.Sprintf("user:%s:events", claims.UserID)
					_ = rdb.SAdd(r.Context(), userEventsKey, ev.ID).Err()
					_ = rdb.Expire(r.Context(), userEventsKey, time.Hour).Err()
				}
			}

			evCtx := &Context{
				EventID: ev.ID,
				Slug:    ev.Slug,
				Name:    ev.Name,
				UserID:  claims.UserID,
				Role:    role,
			}

			// Also synchronize the user's event role into AccessTokenClaims in context so downstream
			// handlers (e.g. tickets, judging, notifications, audit) see the active event role.
			updatedClaims := *claims
			updatedClaims.Role = role
			ctx := auth.WithUserClaims(r.Context(), &updatedClaims)
			ctx = WithEventContext(ctx, evCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireEventRoles verifies that the user's role in the active event matches
// one of the allowed roles.
func RequireEventRoles(allowedRoles ...string) func(http.Handler) http.Handler {
	roleSet := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		roleSet[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			evCtx, ok := FromContext(r.Context())
			if !ok {
				utils.ErrorJSON(w, http.StatusForbidden, "event access context missing")
				return
			}

			// Global dev role has top-level bypass across all event roles
			if evCtx.Role == "dev" {
				next.ServeHTTP(w, r)
				return
			}
			if claims, hasClaims := auth.GetUserClaims(r.Context()); hasClaims && claims.GlobalRole == "dev" {
				next.ServeHTTP(w, r)
				return
			}

			_, allowed := roleSet[evCtx.Role]
			if !allowed && evCtx.Role == "owner" {
				_, allowed = roleSet["admin"]
			}
			if !allowed {
				utils.ErrorJSON(w, http.StatusForbidden, fmt.Sprintf("insufficient event permissions; required one of %v", allowedRoles))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
