package competitions

import (
	"net/http"

	"devclubxnst.com/cadence/internal/modules/auth"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

// RegisterRoutes mounts all competition creation and scoped operations.
func RegisterRoutes(
	r chi.Router,
	h *Handler,
	repo *Repository,
	rdb *redis.Client,
	authMiddleware func(http.Handler) http.Handler,
	extraScopedRoutes ...func(chi.Router),
) {
	// Global competition creation - restricted to devs only
	r.Route("/competitions", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(auth.RequireDev())
		r.Post("/", h.HandleCreateCompetition)
	})

	// Scoped competition routes: /c/{competitionSlug}
	r.Route("/c/{competitionSlug}", func(r chi.Router) {
		r.Use(ResolveCompetition(repo, rdb))

		// Public competition metadata
		r.Get("/", h.HandleGetCompetition)

		// Authenticated member operations
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Use(RequireMembership(repo, rdb))

			// Member identity in this competition
			r.Get("/me", h.HandleGetMyMembership)
			r.Get("/members", h.HandleListMembers)

			// Mount any extra competition-scoped routes (e.g. teams)
			for _, fn := range extraScopedRoutes {
				if fn != nil {
					fn(r)
				}
			}

			// Member operations (owner, admin, volunteer read access to competition users)
			r.Group(func(r chi.Router) {
				r.Use(RequireCompetitionRoles("owner", "admin", "volunteer"))
				r.Get("/users", h.HandleListCompetitionUsers)
			})

			// Admin & Owner management
			r.Group(func(r chi.Router) {
				r.Use(RequireCompetitionRoles("owner", "admin"))

				r.Patch("/", h.HandleUpdateCompetition)
				r.Post("/users", h.HandleAddCompetitionUser)
				r.Post("/users/bulk", h.HandleBulkAddCompetitionUsers)
				r.Post("/members", h.HandleAssignMember)
				r.Patch("/members/{userId}", h.HandleUpdateMemberRole)
				r.Delete("/members/{userId}", h.HandleRemoveMember)
				r.Post("/members/{userId}/ban", h.HandleBanMember)
				r.Post("/members/{userId}/unban", h.HandleUnbanMember)
			})

			// Owner-only operations
			r.Group(func(r chi.Router) {
				r.Use(RequireCompetitionRoles("owner"))

				r.Post("/transfer-ownership", h.HandleTransferOwnership)
			})
		})
	})
}
