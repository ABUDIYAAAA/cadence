package teams

import (
	"net/http"

	"devclubxnst.com/cadence/internal/modules/competitions"
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

// RegisterScopedRoutes mounts teaming endpoints directly inside a competition router (/e/{eventSlug}/c/{competitionSlug}/teams).
func RegisterScopedRoutes(r chi.Router, h *Handler) {
	// Self team lookup (any authenticated member in competition)
	r.Get("/my-team", h.HandleMyTeam)

	// Participant self-service
	r.Group(func(r chi.Router) {
		r.Use(competitions.RequireCompetitionRoles("participant"), validateIDParams)
		r.Post("/", h.HandleCreate)
		r.Post("/join", h.HandleJoin)
		r.Post("/leave", h.HandleLeave)
		r.Patch("/my-team", h.HandleRenameOwn)
		r.Post("/my-team/regenerate-code", h.HandleRegenerateOwnCode)
		r.Delete("/my-team/members/{userId}", h.HandleKick)
		r.Post("/my-team/transfer-leadership", h.HandleTransferLeadership)
	})

	// Read routes: Owner, Admin, Volunteer, Judge
	r.Group(func(r chi.Router) {
		r.Use(competitions.RequireCompetitionRoles("owner", "admin", "volunteer", "judge"), validateIDParams)
		r.Get("/", h.HandleList)
		r.Get("/{id}", h.HandleGet)
	})

	// Staff management: Owner, Admin, Volunteer
	r.Group(func(r chi.Router) {
		r.Use(competitions.RequireCompetitionRoles("owner", "admin", "volunteer"), validateIDParams)
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

	// Admin and Owner only
	r.Group(func(r chi.Router) {
		r.Use(competitions.RequireCompetitionRoles("owner", "admin"), validateIDParams)
		r.Patch("/settings", h.HandleUpdateSettings)
		r.Delete("/{id}", h.HandleDelete)
	})
}
