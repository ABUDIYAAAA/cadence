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

			// Admin & Owner management
			r.Group(func(r chi.Router) {
				r.Use(RequireCompetitionRoles("owner", "admin"))

				r.Patch("/", h.HandleUpdateCompetition)
				r.Post("/members", h.HandleAssignMember)
				r.Patch("/members/{userId}", h.HandleUpdateMemberRole)
				r.Delete("/members/{userId}", h.HandleRemoveMember)
			})

			// Owner-only operations
			r.Group(func(r chi.Router) {
				r.Use(RequireCompetitionRoles("owner"))

				r.Post("/transfer-ownership", h.HandleTransferOwnership)
			})
		})
	})
}
