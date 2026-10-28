package notifications

import (
	"net/http"

	"devclubxnst.com/cadence/internal/modules/auth"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts all notification and Web Push endpoints with role-based gating.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/notifications", func(r chi.Router) {
		// Public/Client key endpoint for browser service worker initialization
		r.Get("/vapid-public-key", h.HandleGetVAPIDPublicKey)

		// Authenticated user routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)

			// Web Push subscriptions
			r.Post("/subscribe", h.HandleSubscribe)
			r.Post("/unsubscribe", h.HandleUnsubscribe)

			// In-App Notification Feed
			r.Get("/inbox", h.HandleGetInbox)
			r.Patch("/{id}/read", h.HandleMarkAsRead)
			r.Post("/read-all", h.HandleMarkAllAsRead)
		})

		// Admin management & broadcast routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Use(auth.RequireRoles("admin"))

			r.Post("/broadcast", h.HandleBroadcast)
			r.Get("/admin/history", h.HandleGetAdminHistory)
		})
	})
}
