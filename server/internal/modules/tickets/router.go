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
			r.Get("/sessions", h.HandleListSessions)
			r.Post("/sessions/{sessionId}/scan", h.HandleSessionScan)
		})

		// Ticket issuance and bulk dispatch: Admin only
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin"))
			r.Post("/bulk-send", h.HandleBulkSend)
			r.Post("/release", h.HandleReleaseTickets)
			r.Get("/settings", h.HandleGetSettings)
			r.Put("/settings", h.HandleUpdateSettings)
			r.Post("/sessions", h.HandleCreateSession)
			r.Patch("/sessions/{sessionId}", h.HandleSetSessionOpen)
			r.Get("/sessions/{sessionId}/report", h.HandleSessionReport)
			r.Post("/users/{userId}/send", h.HandleSendSingle)
		})
	})
}
