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

		// Read routes + participant invites: Admin, Volunteer only (judges evaluate teams via /teams and must not enumerate user directory)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin", "volunteer"))
			r.Get("/", h.HandleList)
			r.Get("/{id}", h.HandleGet)

			// Onboarding emails. Volunteers are limited to participant accounts in the service.
			r.Post("/{id}/send-invite", h.HandleSendInvite)
			r.Post("/bulk-send-invite", h.HandleBulkSendInvite)
		})

		// Admin-only management routes
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin"))
			r.Post("/bulk", h.HandleBulkCreate)
			r.Post("/", h.HandleCreate)
			r.Patch("/{id}", h.HandleUpdate)
			r.Delete("/{id}", h.HandleDelete)

			// Recovery triggers
			r.Post("/{id}/trigger-reset-password", h.HandleTriggerResetPassword)
		})
	})
}
