package judging

import (
	"net/http"

	"devclubxnst.com/cadence/internal/modules/auth"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts all judging, scoring, and queue tracking endpoints with role-based access control.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/judging", func(r chi.Router) {
		r.Use(authMiddleware)

		// Participant & Team routes
		r.Get("/teams/my-status", h.HandleGetTeamStatus)
		r.Patch("/slots/{slotId}/links", h.HandleUpdateSlotLinks)

		// Volunteer coordinator routes
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("volunteer", "admin", "judge"))
			r.Get("/volunteer/schedule", h.HandleGetVolunteerSchedule)
		})

		// Judge evaluation & scoring routes
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("judge", "admin"))
			r.Get("/judge/panels", h.HandleGetJudgePanels)
			r.Get("/slots/{slotId}", h.HandleGetJudgeSlotDetails)
			r.Post("/slots/{slotId}/start", h.HandleStartSlot)
			r.Post("/slots/{slotId}/scores", h.HandleSubmitScores)
			r.Post("/slots/{slotId}/lock", h.HandleLockScores)
			r.Post("/slots/{slotId}/complete", h.HandleCompleteSlot)
		})

		// Judging rounds registry & criteria rubrics (accessible to admin, judge, and volunteer)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin", "judge", "volunteer"))
			r.Get("/rounds", h.HandleListRounds)
			r.Get("/rounds/{id}", h.HandleGetRound)
		})

		// Admin management & leaderboard routes
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles("admin"))
			r.Post("/rounds", h.HandleCreateRound)
			r.Post("/rounds/{id}/activate", h.HandleActivateRound)
			r.Get("/rounds/{id}/leaderboard", h.HandleGetAdminLeaderboard)
			r.Post("/panels", h.HandleCreatePanel)
			r.Post("/panels/{panelId}/slots", h.HandleConfigureSlots)
			r.Post("/slots/{slotId}/move", h.HandleMoveSlot)
			r.Post("/slots/{slotId}/queue", h.HandleQueueSlot)
			r.Post("/slots/{slotId}/unqueue", h.HandleUnqueueSlot)
		})
	})
}
