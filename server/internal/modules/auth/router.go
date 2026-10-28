package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes registers authentication endpoints on the provided chi router.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/auth", func(r chi.Router) {
		// Public routes
		r.Post("/login", h.HandleLogin)
		r.Post("/setup-password", h.HandleSetupPassword)
		r.Post("/forgot-password", h.HandleForgotPassword)
		r.Post("/reset-password", h.HandleResetPassword)
		r.Post("/refresh", h.HandleRefreshToken)
		r.Post("/logout", h.HandleLogout)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Get("/me", h.HandleMe)
		})
	})
}
