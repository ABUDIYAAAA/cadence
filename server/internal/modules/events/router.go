package events

import (
	"net/http"

	"devclubxnst.com/cadence/internal/modules/auth"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

// RegisterRoutes mounts all event creation, deletion, and event-scoped operations.
func RegisterRoutes(
	r chi.Router,
	h *Handler,
	repo *Repository,
	rdb *redis.Client,
	authMiddleware func(http.Handler) http.Handler,
	extraScopedRoutes ...func(chi.Router),
) {
	// Global event management - creation and deletion restricted strictly to devs
	r.Route("/events", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/", h.HandleListEvents)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireDev())
			r.Post("/", h.HandleCreateEvent)
			r.Delete("/{eventSlug}", h.HandleDeleteEvent)
		})
	})

	// Scoped event routes: /e/{eventSlug}
	r.Route("/e/{eventSlug}", func(r chi.Router) {
		r.Use(ResolveEvent(repo, rdb))

		// Public event metadata
		r.Get("/", h.HandleGetEvent)

		// Dev-only event deletion via scoped URL
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Use(auth.RequireDev())
			r.Delete("/", h.HandleDeleteEvent)
		})

		// Authenticated event member operations
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Use(RequireMembership(repo, rdb))

			// Member identity in this event
			r.Get("/me", h.HandleGetMyMembership)
			r.Get("/members", h.HandleListMembers)

			// Mount any extra event-scoped routes (e.g. competitions, tickets, judging, notifications, audit)
			for _, fn := range extraScopedRoutes {
				if fn != nil {
					fn(r)
				}
			}

			// Member operations (owner, admin, volunteer read access to event users)
			r.Group(func(r chi.Router) {
				r.Use(RequireEventRoles("owner", "admin", "volunteer"))
				r.Get("/users", h.HandleListEventUsers)
			})

			// Admin & Owner management
			r.Group(func(r chi.Router) {
				r.Use(RequireEventRoles("owner", "admin"))

				r.Patch("/", h.HandleUpdateEvent)
				r.Post("/users", h.HandleAddEventUser)
				r.Post("/users/bulk", h.HandleBulkAddEventUsers)
				r.Post("/members", h.HandleAssignMember)
				r.Patch("/members/{userId}", h.HandleUpdateMemberRole)
				r.Delete("/members/{userId}", h.HandleRemoveMember)
				r.Post("/members/{userId}/ban", h.HandleBanMember)
				r.Post("/members/{userId}/unban", h.HandleUnbanMember)
			})

			// Owner-only operations
			r.Group(func(r chi.Router) {
				r.Use(RequireEventRoles("owner"))

				r.Post("/transfer-ownership", h.HandleTransferOwnership)
			})
		})
	})
}
