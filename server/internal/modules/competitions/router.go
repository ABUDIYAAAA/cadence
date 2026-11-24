package competitions

import (
	"devclubxnst.com/cadence/internal/modules/events"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

// RegisterEventScopedRoutes mounts competition CRUD and competition-scoped sub-routes inside an event router (/e/{eventSlug}).
// Assumes events.ResolveEvent and events.RequireMembership have already run on the parent router.
func RegisterEventScopedRoutes(
	r chi.Router,
	h *Handler,
	repo *Repository,
	rdb *redis.Client,
	extraCompetitionScopedRoutes ...func(chi.Router),
) {
	// Event-level competition collection: /e/{eventSlug}/competitions
	r.Route("/competitions", func(r chi.Router) {
		r.Get("/", h.HandleListCompetitions)

		// Only event owners, admins, and devs can create competitions
		r.Group(func(r chi.Router) {
			r.Use(events.RequireEventRoles("owner", "admin"))
			r.Post("/", h.HandleCreateCompetition)
		})
	})

	// Scoped competition routes: /e/{eventSlug}/c/{competitionSlug}
	r.Route("/c/{competitionSlug}", func(r chi.Router) {
		r.Use(ResolveCompetition(repo, rdb))

		r.Get("/", h.HandleGetCompetition)

		// Mount competition-scoped sub-modules (e.g. /teams, /judging)
		for _, fn := range extraCompetitionScopedRoutes {
			if fn != nil {
				fn(r)
			}
		}

		// Only event owners, admins, and devs can edit or delete competitions
		r.Group(func(r chi.Router) {
			r.Use(RequireCompetitionRoles("owner", "admin"))
			r.Patch("/", h.HandleUpdateCompetition)
			r.Delete("/", h.HandleDeleteCompetition)
		})
	})
}
