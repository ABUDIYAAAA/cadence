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

		// Self-service: Any authenticated user can view their ticket pass or check in via an active check-in / attendance session code
		r.Get("/me", h.HandleGetMyTicket)
		r.Post("/check-in/code", h.HandleCheckInByCode)
		r.Post("/sessions/check-in/code", h.HandleCheckInByCode)

		// Scanner and analytics: Admin, Owner, Volunteer, Judge
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin", "volunteer", "judge"))
			r.Post("/check-in", h.HandleCheckIn)
			r.Get("/analytics", h.HandleGetAnalytics)
			r.Get("/sessions", h.HandleListSessions)
			r.Post("/sessions/{sessionId}/scan", h.HandleSessionScan)
			r.Get("/sessions/{sessionId}/report", h.HandleSessionReport)
		})

		// Ticket issuance, bulk dispatch, scan session creation, and check-in code generation: Admin / Owner only
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin"))
			r.Post("/bulk-send", h.HandleBulkSend)
			r.Post("/release", h.HandleReleaseTickets)
			r.Get("/settings", h.HandleGetSettings)
			r.Put("/settings", h.HandleUpdateSettings)
			r.Post("/checkin-code", h.HandleGenerateDefaultCheckInCode)
			r.Delete("/checkin-code", h.HandleClearDefaultCheckInCode)
			r.Post("/sessions", h.HandleCreateSession)
			r.Patch("/sessions/{sessionId}", h.HandleSetSessionOpen)
			r.Post("/sessions/{sessionId}/code", h.HandleGenerateSessionCode)
			r.Delete("/sessions/{sessionId}/code", h.HandleClearSessionCode)
			r.Post("/users/{userId}/send", h.HandleSendSingle)
		})
	})
}
