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
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to activate round: "+err.Error())
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
		utils.ErrorJSON(w, http.StatusBadRequest, "failed to configure slots: "+err.Error())
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

	if err := h.service.UpdateSlotLinks(r.Context(), slotID, req.SubmissionLinks); err != nil {
		if errors.Is(err, ErrSlotNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "judging slot not found")
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
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to start slot: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "Judging slot pitch started successfully",
		"slot_id": slotID,
	})
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
	schedule, err := h.service.GetVolunteerSchedule(r.Context())
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
