package teams

import (
	"net/http"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// validateIDParams rejects malformed {id}/{userId} path params before they reach the database.
func validateIDParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, name := range []string{"id", "userId"} {
			if v := chi.URLParam(r, name); v != "" {
				if _, err := uuid.Parse(v); err != nil {
					utils.ErrorJSON(w, http.StatusNotFound, "not found")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RegisterRoutes mounts all teams endpoints (API v2) with role gating.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/teams", func(r chi.Router) {
		r.Use(authMiddleware)

		// Self team lookup (any signed-in user; non-participants simply have no team)
		r.Get("/my-team", h.HandleMyTeam)

		// Participant self-service
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("participant"), validateIDParams)
			r.Post("/", h.HandleCreate)
			r.Post("/join", h.HandleJoin)
			r.Post("/leave", h.HandleLeave)
			r.Patch("/my-team", h.HandleRenameOwn)
			r.Post("/my-team/regenerate-code", h.HandleRegenerateOwnCode)
			r.Delete("/my-team/members/{userId}", h.HandleKick)
		})

		// Read routes: Admin, Volunteer, Judge
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin", "volunteer", "judge"), validateIDParams)
			r.Get("/", h.HandleList)
			r.Get("/{id}", h.HandleGet)
		})

		// Staff management: Admin, Volunteer
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin", "volunteer"), validateIDParams)
			r.Get("/settings", h.HandleGetSettings)
			r.Post("/freeze-all", h.HandleFreezeAll)
			r.Post("/unfreeze-all", h.HandleUnfreezeAll)
			r.Patch("/{id}", h.HandleUpdate)
			r.Post("/{id}/freeze", h.HandleFreeze)
			r.Post("/{id}/unfreeze", h.HandleUnfreeze)
			r.Post("/{id}/regenerate-code", h.HandleRegenerateCode)
			r.Post("/{id}/members", h.HandleAddMember)
			r.Delete("/{id}/members/{userId}", h.HandleRemoveMember)
		})

		// Admin-only
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin"), validateIDParams)
			r.Patch("/settings", h.HandleUpdateSettings)
			r.Delete("/{id}", h.HandleDelete)
		})
	})
}
