package users

import (
	"net/http"

	"devclubxnst.com/cadence/internal/modules/auth"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts all users endpoints with role gating.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/users", func(r chi.Router) {
		r.Use(authMiddleware)

		// Read routes: Admin, Volunteer, Judge
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin", "volunteer", "judge"))
			r.Get("/", h.HandleList)
			r.Get("/{id}", h.HandleGet)
		})

		// Admin-only management routes
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin"))
			r.Post("/bulk", h.HandleBulkCreate)
			r.Post("/", h.HandleCreate)
			r.Patch("/{id}", h.HandleUpdate)
			r.Delete("/{id}", h.HandleDelete)

			// Email onboarding & recovery triggers
			r.Post("/{id}/send-invite", h.HandleSendInvite)
			r.Post("/bulk-send-invite", h.HandleBulkSendInvite)
			r.Post("/{id}/trigger-reset-password", h.HandleTriggerResetPassword)
		})
	})
}
