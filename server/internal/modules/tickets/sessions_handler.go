package tickets

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/pkg/utils"
)

func (h *Handler) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if errs := utils.ValidateStruct(&req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}
	sess, err := h.service.CreateSession(r.Context(), req, claims.UserID, claims.Email, auth.GetEffectiveRole(r.Context()), r.RemoteAddr, r.UserAgent())
	if err != nil {
		if errors.Is(err, ErrCompetitionRequiredForTeamScan) || errors.Is(err, ErrInvalidCompetition) || errors.Is(err, ErrInvalidCheckInCode) {
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, ErrCheckInCodeConflict) {
			utils.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to create scan session: "+err.Error())
		return
	}
	utils.JSON(w, http.StatusCreated, sess)
}

func (h *Handler) HandleGenerateDefaultCheckInCode(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req GenerateCheckInCodeRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	sessionID := ""
	if req.SessionID != nil {
		sessionID = *req.SessionID
	}
	sess, err := h.service.GenerateSessionCheckInCode(r.Context(), sessionID, req.Code, claims.UserID, claims.Email, auth.GetEffectiveRole(r.Context()), r.RemoteAddr, r.UserAgent())
	if err != nil {
		switch {
		case errors.Is(err, ErrSessionNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, "scan session not found")
		case errors.Is(err, ErrInvalidCheckInCode):
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrCheckInCodeConflict):
			utils.ErrorJSON(w, http.StatusConflict, err.Error())
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to generate check-in code: "+err.Error())
		}
		return
	}
	utils.JSON(w, http.StatusOK, sess)
}

func (h *Handler) HandleClearDefaultCheckInCode(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sess, err := h.service.ClearSessionCheckInCode(r.Context(), "", claims.UserID, claims.Email, auth.GetEffectiveRole(r.Context()), r.RemoteAddr, r.UserAgent())
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to clear check-in code: "+err.Error())
		return
	}
	utils.JSON(w, http.StatusOK, sess)
}

func (h *Handler) HandleGenerateSessionCode(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req GenerateCheckInCodeRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	sess, err := h.service.GenerateSessionCheckInCode(r.Context(), chi.URLParam(r, "sessionId"), req.Code, claims.UserID, claims.Email, auth.GetEffectiveRole(r.Context()), r.RemoteAddr, r.UserAgent())
	if err != nil {
		switch {
		case errors.Is(err, ErrSessionNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, "scan session not found")
		case errors.Is(err, ErrInvalidCheckInCode):
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrCheckInCodeConflict):
			utils.ErrorJSON(w, http.StatusConflict, err.Error())
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to generate check-in code: "+err.Error())
		}
		return
	}
	utils.JSON(w, http.StatusOK, sess)
}

func (h *Handler) HandleClearSessionCode(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	sess, err := h.service.ClearSessionCheckInCode(r.Context(), chi.URLParam(r, "sessionId"), claims.UserID, claims.Email, auth.GetEffectiveRole(r.Context()), r.RemoteAddr, r.UserAgent())
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "scan session not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to clear check-in code: "+err.Error())
		return
	}
	utils.JSON(w, http.StatusOK, sess)
}

func (h *Handler) HandleCheckInByCode(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req CheckInByCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if errs := utils.ValidateStruct(&req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	resp, rejection, err := h.service.CheckInByCode(r.Context(), req.Code, claims.UserID, claims.Email, auth.GetEffectiveRole(r.Context()), r.RemoteAddr, r.UserAgent())
	switch {
	case err == nil:
		utils.JSON(w, http.StatusOK, resp)
	case errors.Is(err, ErrInvalidCheckInCode):
		utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNotEventMember):
		utils.ErrorJSON(w, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrAlreadyScanned), errors.Is(err, ErrSessionClosed):
		utils.JSON(w, http.StatusConflict, rejection)
	case errors.Is(err, ErrNotCheckedIn), errors.Is(err, ErrExitNotRequired):
		utils.JSON(w, http.StatusUnprocessableEntity, rejection)
	default:
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to check in with code: "+err.Error())
	}
}

func (h *Handler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.service.ListSessions(r.Context())
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to list scan sessions: "+err.Error())
		return
	}
	utils.JSON(w, http.StatusOK, sessions)
}

func (h *Handler) HandleSetSessionOpen(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req SetSessionOpenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sess, err := h.service.SetSessionOpen(r.Context(), chi.URLParam(r, "sessionId"), req.IsOpen,
		claims.UserID, claims.Email, auth.GetEffectiveRole(r.Context()), r.RemoteAddr, r.UserAgent())
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "scan session not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to update scan session: "+err.Error())
		return
	}
	utils.JSON(w, http.StatusOK, sess)
}

func (h *Handler) HandleSessionScan(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req SessionScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if errs := utils.ValidateStruct(&req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	resp, rejection, err := h.service.ScanInSession(r.Context(), chi.URLParam(r, "sessionId"), req.TicketCode,
		claims.UserID, claims.Email, auth.GetEffectiveRole(r.Context()), r.RemoteAddr, r.UserAgent())
	switch {
	case err == nil:
		utils.JSON(w, http.StatusOK, resp)
	case errors.Is(err, ErrSessionNotFound):
		utils.ErrorJSON(w, http.StatusNotFound, "scan session not found")
	case errors.Is(err, ErrTicketNotFound):
		utils.ErrorJSON(w, http.StatusNotFound, "ticket not found")
	case errors.Is(err, ErrAlreadyScanned), errors.Is(err, ErrSessionClosed):
		utils.JSON(w, http.StatusConflict, rejection)
	case errors.Is(err, ErrNotCheckedIn), errors.Is(err, ErrExitNotRequired):
		utils.JSON(w, http.StatusUnprocessableEntity, rejection)
	default:
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to record scan: "+err.Error())
	}
}

func (h *Handler) HandleSessionReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.service.GetSessionReport(r.Context(), chi.URLParam(r, "sessionId"))
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "scan session not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to build session report: "+err.Error())
		return
	}
	utils.JSON(w, http.StatusOK, report)
}

// HandleReleaseTickets creates passes and shows them on the participant dashboard without any email.
func (h *Handler) HandleReleaseTickets(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req BulkSendTicketsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = BulkSendTicketsRequest{}
	}
	resp, err := h.service.ReleaseTickets(r.Context(), req, claims.UserID, claims.Email, auth.GetEffectiveRole(r.Context()), r.RemoteAddr, r.UserAgent())
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to release tickets: "+err.Error())
		return
	}
	utils.JSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetSettings(r.Context())
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to load ticket settings: "+err.Error())
		return
	}
	utils.JSON(w, http.StatusOK, settings)
}

func (h *Handler) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req UpdateTicketSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	settings, err := h.service.UpdateSettings(r.Context(), req, claims.UserID, claims.Email, auth.GetEffectiveRole(r.Context()), r.RemoteAddr, r.UserAgent())
	if err != nil {
		if errors.Is(err, ErrInvalidDomain) || errors.Is(err, ErrInvalidEmailSubject) {
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to save ticket settings: "+err.Error())
		return
	}
	utils.JSON(w, http.StatusOK, settings)
}
