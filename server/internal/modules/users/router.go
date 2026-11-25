package users

import (
	"net/http"

	"devclubxnst.com/cadence/internal/modules/auth"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts all users endpoints with role gating (restricted strictly to devs).
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/users", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(auth.RequireDev())

		r.Get("/", h.HandleList)
		r.Get("/{id}", h.HandleGet)
		r.Post("/", h.HandleCreate)
		r.Post("/bulk", h.HandleBulkCreate)
		r.Patch("/{id}", h.HandleUpdate)
		r.Delete("/{id}", h.HandleDelete)
		r.Post("/{id}/reactivate", h.HandleRestore)
		r.Post("/{id}/blacklist", h.HandleBlacklist)
		r.Post("/{id}/unblacklist", h.HandleUnblacklist)
	})
}
