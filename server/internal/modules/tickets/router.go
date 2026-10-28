package tickets

import (
	"net/http"

	"devclubxnst.com/cadence/internal/modules/auth"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts all ticketing and attendance endpoints with role gating.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/tickets", func(r chi.Router) {
		r.Use(authMiddleware)

		// Self-service: Any authenticated user can view their ticket pass
		r.Get("/me", h.HandleGetMyTicket)

		// Scanner and analytics: Admin, Volunteer, Judge
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin", "volunteer", "judge"))
			r.Post("/check-in", h.HandleCheckIn)
			r.Get("/analytics", h.HandleGetAnalytics)
		})

		// Ticket issuance and bulk dispatch: Admin only
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin"))
			r.Post("/bulk-send", h.HandleBulkSend)
			r.Post("/users/{userId}/send", h.HandleSendSingle)
		})
	})
}
