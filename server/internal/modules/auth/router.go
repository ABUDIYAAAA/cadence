package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes registers authentication endpoints on the provided chi router.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/auth", func(r chi.Router) {
		// Public Google OAuth routes
		r.Get("/google/login", h.HandleGoogleLogin)
		r.Get("/google/callback", h.HandleGoogleCallback)

		// Session management
		r.Post("/refresh", h.HandleRefreshToken)
		r.Post("/logout", h.HandleLogout)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Get("/me", h.HandleMe)
		})
	})
}
