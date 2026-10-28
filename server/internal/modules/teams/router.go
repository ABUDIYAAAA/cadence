package teams

import (
	"net/http"

	"devclubxnst.com/cadence/internal/modules/auth"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts all teams endpoints with role gating.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/teams", func(r chi.Router) {
		r.Use(authMiddleware)

		// Self team lookup (participants, judges, volunteers, admins)
		r.Get("/my-team", h.HandleMyTeam)

		// Read routes: Admin, Volunteer, Judge
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin", "volunteer", "judge"))
			r.Get("/", h.HandleList)
			r.Get("/{id}", h.HandleGet)
		})

		// Admin-only team management
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin"))
			r.Post("/", h.HandleCreate)
			r.Patch("/{id}", h.HandleUpdate)
			r.Delete("/{id}", h.HandleDelete)

			// Team member management
			r.Post("/{id}/members", h.HandleAddMember)
			r.Delete("/{id}/members/{userId}", h.HandleRemoveMember)
		})
	})
}
