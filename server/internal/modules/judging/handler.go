package judging

import (
	"encoding/json"
	"errors"
	"net/http"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// HandleCreateRound creates a judging round with criteria.
func (h *Handler) HandleCreateRound(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateRoundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(&req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	roundID, err := h.service.CreateRound(r.Context(), req, claims.UserID, claims.Email, claims.Role, ip, ua)
	if err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.JSON(w, http.StatusCreated, map[string]any{
		"message":  "Judging round created successfully",
		"round_id": roundID,
	})
}

// HandleActivateRound transitions round status to active.
func (h *Handler) HandleActivateRound(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	roundID := chi.URLParam(r, "id")
	if roundID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "round ID is required")
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	if err := h.service.ActivateRound(r.Context(), roundID, claims.UserID, claims.Email, claims.Role, ip, ua); err != nil {
		switch {
		case errors.Is(err, ErrRoundNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, "judging round not found")
		case errors.Is(err, ErrRoundFinished):
			utils.ErrorJSON(w, http.StatusConflict, err.Error())
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to activate round: "+err.Error())
		}
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message":  "Judging round activated successfully",
		"round_id": roundID,
	})
}

// HandleListRounds lists all judging rounds.
func (h *Handler) HandleListRounds(w http.ResponseWriter, r *http.Request) {
	rounds, err := h.service.ListRounds(r.Context())
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to list rounds: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, rounds)
}

// HandleGetRound returns details and criteria for a single round.
func (h *Handler) HandleGetRound(w http.ResponseWriter, r *http.Request) {
	roundID := chi.URLParam(r, "id")
	if roundID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "round ID is required")
		return
	}

	req, status, bufferSize, duration, criteria, err := h.service.GetRound(r.Context(), roundID)
	if err != nil {
		if errors.Is(err, ErrRoundNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "judging round not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]any{
		"id":                 roundID,
		"name":               req.Name,
		"status":             status,
		"buffer_size":        bufferSize,
		"slot_duration_mins": duration,
		"criteria":           criteria,
	})
}

// HandleCreatePanel creates a judging panel in a room and assigns judges.
// HandleGetRoundPanels lists a round's panels and the teams each already holds.
func (h *Handler) HandleGetRoundPanels(w http.ResponseWriter, r *http.Request) {
	panels, err := h.service.GetRoundPanels(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to fetch round panels")
		return
	}
	utils.JSON(w, http.StatusOK, panels)
}

// HandleGetAssignedJudges lists judges already seated on a panel in the round.
func (h *Handler) HandleGetAssignedJudges(w http.ResponseWriter, r *http.Request) {
	roundID := chi.URLParam(r, "id")
	judges, err := h.service.GetAssignedJudges(r.Context(), roundID)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to fetch assigned judges")
		return
	}
	utils.JSON(w, http.StatusOK, judges)
}

func (h *Handler) HandleCreatePanel(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreatePanelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(&req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	panelID, err := h.service.CreatePanel(r.Context(), req, claims.UserID, claims.Email, claims.Role, ip, ua)
	if err != nil {
		if errors.Is(err, ErrJudgeAssigned) {
			utils.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		utils.ErrorJSON(w, http.StatusBadRequest, "failed to create panel: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusCreated, map[string]any{
		"message":  "Judging panel created successfully",
		"panel_id": panelID,
	})
}

// HandleConfigureSlots sets sequential team schedule in a panel.
func (h *Handler) HandleConfigureSlots(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	panelID := chi.URLParam(r, "panelId")
	if panelID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "panel ID is required")
		return
	}

	var req ConfigureSlotsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(&req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	if err := h.service.ConfigureSlots(r.Context(), panelID, req, claims.UserID, claims.Email, claims.Role, ip, ua); err != nil {
		switch {
		case errors.Is(err, ErrPanelNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, "judging panel not found")
		case errors.Is(err, ErrTeamSlotted), errors.Is(err, ErrPanelStarted):
			utils.ErrorJSON(w, http.StatusConflict, err.Error())
		default:
			utils.ErrorJSON(w, http.StatusBadRequest, "failed to configure slots: "+err.Error())
		}
		return
	}

	utils.JSON(w, http.StatusOK, map[string]any{
		"message":    "Panel slots configured successfully",
		"panel_id":   panelID,
		"slot_count": len(req.Slots),
	})
}

// HandleUpdateSlotLinks updates repository, presentation, or PRD links for a slot.
func (h *Handler) HandleUpdateSlotLinks(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	slotID := chi.URLParam(r, "slotId")
	if slotID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "slot ID is required")
		return
	}

	var req UpdateSlotLinksRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(&req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	if err := h.service.UpdateSlotLinks(r.Context(), slotID, claims.UserID, claims.Role, req.SubmissionLinks); err != nil {
		if errors.Is(err, ErrSlotNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "judging slot not found")
			return
		}
		if errors.Is(err, ErrUnauthorizedSlot) {
			utils.ErrorJSON(w, http.StatusForbidden, "you can only update submission links for your own team")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to update links: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "Slot submission links updated successfully",
		"slot_id": slotID,
	})
}

// HandleGetJudgePanels returns panels where the authenticated judge is assigned.
func (h *Handler) HandleGetJudgePanels(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	panels, err := h.service.GetJudgePanels(r.Context(), claims.UserID)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to fetch judge panels: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, panels)
}

// HandleGetJudgeSlotDetails returns the full evaluation view for a single slot with countdown timer data.
func (h *Handler) HandleGetJudgeSlotDetails(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	slotID := chi.URLParam(r, "slotId")
	if slotID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "slot ID is required")
		return
	}

	details, err := h.service.GetJudgeSlotDetails(r.Context(), slotID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrSlotNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "judging slot not found")
			return
		}
		if errors.Is(err, ErrJudgeNotAssigned) {
			utils.ErrorJSON(w, http.StatusForbidden, "you are not assigned to evaluate this judging panel")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to get slot details: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, details)
}

// HandleStartSlot initiates a team's pitch and marks upcoming teams as queued.
func (h *Handler) HandleStartSlot(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	slotID := chi.URLParam(r, "slotId")
	if slotID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "slot ID is required")
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	if err := h.service.StartSlot(r.Context(), slotID, claims.UserID, claims.Email, claims.Role, ip, ua); err != nil {
		if errors.Is(err, ErrSlotNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "judging slot not found")
			return
		}
		if errors.Is(err, ErrJudgeNotAssigned) {
			utils.ErrorJSON(w, http.StatusForbidden, "you are not assigned to evaluate this judging panel")
			return
		}
		if errors.Is(err, ErrSlotNotPending) || errors.Is(err, ErrRoundNotActive) || errors.Is(err, ErrPanelBusy) {
			utils.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to start slot: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "Judging slot pitch started successfully",
		"slot_id": slotID,
	})
}

// timerActionError maps timer lifecycle errors onto HTTP responses.
func timerActionError(w http.ResponseWriter, err error, verb string) {
	switch {
	case errors.Is(err, ErrSlotNotFound):
		utils.ErrorJSON(w, http.StatusNotFound, "judging slot not found")
	case errors.Is(err, ErrJudgeNotAssigned):
		utils.ErrorJSON(w, http.StatusForbidden, "you are not assigned to evaluate this judging panel")
	case errors.Is(err, ErrSlotNotStarted), errors.Is(err, ErrTimerPaused), errors.Is(err, ErrTimerNotPaused), errors.Is(err, ErrPitchEnded):
		utils.ErrorJSON(w, http.StatusConflict, err.Error())
	default:
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to "+verb+" timer: "+err.Error())
	}
}

// HandlePauseTimer pauses the countdown for a running pitch.
func (h *Handler) HandlePauseTimer(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	slotID := chi.URLParam(r, "slotId")
	if err := h.service.PauseTimer(r.Context(), slotID, claims.UserID, claims.Email, claims.Role, r.RemoteAddr, r.UserAgent()); err != nil {
		timerActionError(w, err, "pause")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{"message": "Timer paused", "slot_id": slotID})
}

// HandleResumeTimer resumes a paused countdown.
func (h *Handler) HandleResumeTimer(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	slotID := chi.URLParam(r, "slotId")
	if err := h.service.ResumeTimer(r.Context(), slotID, claims.UserID, claims.Email, claims.Role, r.RemoteAddr, r.UserAgent()); err != nil {
		timerActionError(w, err, "resume")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{"message": "Timer resumed", "slot_id": slotID})
}

// HandleExtendTimer adds extra time to a running pitch.
func (h *Handler) HandleExtendTimer(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	slotID := chi.URLParam(r, "slotId")

	var req ExtendTimerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if errs := utils.ValidateStruct(&req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	if err := h.service.ExtendTimer(r.Context(), slotID, claims.UserID, claims.Email, claims.Role, req.Seconds, r.RemoteAddr, r.UserAgent()); err != nil {
		timerActionError(w, err, "extend")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{"message": "Timer extended", "slot_id": slotID})
}

// HandleSubmitScores records criteria scores for a judge without locking yet.
func (h *Handler) HandleSubmitScores(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	slotID := chi.URLParam(r, "slotId")
	if slotID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "slot ID is required")
		return
	}

	var req SubmitScoresRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(&req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	if err := h.service.SubmitScores(r.Context(), slotID, claims.UserID, req); err != nil {
		if errors.Is(err, ErrSlotNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "judging slot not found")
			return
		}
		if errors.Is(err, ErrJudgeNotAssigned) {
			utils.ErrorJSON(w, http.StatusForbidden, "you are not assigned to evaluate this judging panel")
			return
		}
		if errors.Is(err, ErrSlotNotStarted) {
			utils.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "Criteria scores saved successfully",
		"slot_id": slotID,
	})
}

// HandleLockScores locks the judge's score for the team.
func (h *Handler) HandleLockScores(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	slotID := chi.URLParam(r, "slotId")
	if slotID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "slot ID is required")
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	if err := h.service.LockScores(r.Context(), slotID, claims.UserID, claims.Email, claims.Role, ip, ua); err != nil {
		if errors.Is(err, ErrSlotNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "judging slot not found")
			return
		}
		if errors.Is(err, ErrJudgeNotAssigned) {
			utils.ErrorJSON(w, http.StatusForbidden, "you are not assigned to evaluate this judging panel")
			return
		}
		if errors.Is(err, ErrSlotNotStarted) {
			utils.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrIncompleteScore) {
			utils.ErrorJSON(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "Evaluation scorecard locked successfully",
		"slot_id": slotID,
	})
}

// HandleCompleteSlot advances the queue once all judges on the panel have locked scores.
func (h *Handler) HandleCompleteSlot(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	slotID := chi.URLParam(r, "slotId")
	if slotID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "slot ID is required")
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	if err := h.service.CompleteSlot(r.Context(), slotID, claims.UserID, claims.Email, claims.Role, ip, ua); err != nil {
		if errors.Is(err, ErrSlotNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "judging slot not found")
			return
		}
		if errors.Is(err, ErrJudgeNotAssigned) {
			utils.ErrorJSON(w, http.StatusForbidden, "you are not assigned to evaluate this judging panel")
			return
		}
		if errors.Is(err, ErrSlotNotStarted) {
			utils.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrNotAllJudgesLocked) {
			utils.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to complete slot: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "Judging slot completed and queue advanced successfully",
		"slot_id": slotID,
	})
}

// HandleGetTeamStatus participant view displaying live room coordinates and queue index.
func (h *Handler) HandleGetTeamStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	resp, err := h.service.GetTeamStatusForUser(r.Context(), claims.UserID)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to get team status: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, resp)
}

// HandleGetVolunteerSchedule room schedule view for event coordinators.
func (h *Handler) HandleGetVolunteerSchedule(w http.ResponseWriter, r *http.Request) {
	var judgeID *string
	if claims, ok := auth.GetUserClaims(r.Context()); ok && claims.Role == "judge" {
		judgeID = &claims.UserID
	}

	schedule, err := h.service.GetVolunteerSchedule(r.Context(), judgeID)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to fetch volunteer schedule: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, schedule)
}

// HandleGetAdminLeaderboard calculates full round rankings with weighted criteria breakdown.
func (h *Handler) HandleGetAdminLeaderboard(w http.ResponseWriter, r *http.Request) {
	roundID := chi.URLParam(r, "id")
	if roundID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "round ID is required")
		return
	}

	leaderboard, err := h.service.GetAdminLeaderboard(r.Context(), roundID)
	if err != nil {
		if errors.Is(err, ErrRoundNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "judging round not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to compute leaderboard: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, leaderboard)
}

// HandleMoveSlot moves a team slot across panels or reorders inside a panel.
func (h *Handler) HandleMoveSlot(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	slotID := chi.URLParam(r, "slotId")
	if slotID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "slot ID is required")
		return
	}

	var req MoveSlotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	if err := h.service.MoveSlot(r.Context(), slotID, req, claims.UserID, claims.Email, claims.Role, ip, ua); err != nil {
		if errors.Is(err, ErrSlotNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "judging slot not found")
			return
		}
		if errors.Is(err, ErrSlotLive) {
			utils.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "Team slot moved and room queue updated successfully",
	})
}

// HandleQueueSlot immediately stages a team slot into the room queue ('queued' status).
func (h *Handler) HandleQueueSlot(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	slotID := chi.URLParam(r, "slotId")
	if slotID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "slot ID is required")
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	if err := h.service.QueueSlot(r.Context(), slotID, claims.UserID, claims.Email, claims.Role, ip, ua); err != nil {
		if errors.Is(err, ErrSlotNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "judging slot not found")
			return
		}
		utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "Team successfully queued on-deck for room evaluation",
	})
}

// HandleUnqueueSlot moves a team slot from 'queued' back to 'scheduled'.
func (h *Handler) HandleUnqueueSlot(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	slotID := chi.URLParam(r, "slotId")
	if slotID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "slot ID is required")
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	if err := h.service.UnqueueSlot(r.Context(), slotID, claims.UserID, claims.Email, claims.Role, ip, ua); err != nil {
		if errors.Is(err, ErrSlotNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "judging slot not found")
			return
		}
		utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "Team reverted to scheduled slot",
	})
}
