package audit

import (
	"net/http"

	"devclubxnst.com/cadence/internal/modules/auth"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes registers event-scoped audit log endpoints onto the chi router, gated to administrators.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/audit-logs", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(auth.RequireRoles("admin"))

		r.Get("/", h.HandleList)
		r.Get("/export", h.HandleExport)
	})
}

// RegisterGlobalRoutes registers platform-wide audit log endpoints gated strictly to devs.
func RegisterGlobalRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/audit-logs", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(auth.RequireDev())

		r.Get("/", h.HandleList)
		r.Get("/export", h.HandleExport)
	})
}
