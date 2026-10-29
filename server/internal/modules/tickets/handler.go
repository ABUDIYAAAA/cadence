package tickets

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

// HandleCheckIn handles 1-scan check-in with duplicate conflict reporting.
func (h *Handler) HandleCheckIn(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CheckInRequest
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

	resp, duplicateResp, err := h.service.CheckIn(r.Context(), claims.UserID, claims.Email, claims.Role, req.TicketCode, ip, ua)
	if err != nil {
		if errors.Is(err, ErrAlreadyCheckedIn) {
			utils.JSON(w, http.StatusConflict, duplicateResp)
			return
		}
		if errors.Is(err, ErrTicketNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "ticket not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to process check-in: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, resp)
}

// HandleSendSingle triggers immediate ticket generation & email delivery to a specific user.
func (h *Handler) HandleSendSingle(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	targetUserID := chi.URLParam(r, "userId")
	if targetUserID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "user ID is required")
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	resp, err := h.service.SendSingleTicket(r.Context(), targetUserID, claims.UserID, claims.Email, claims.Role, ip, ua)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to dispatch ticket: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, resp)
}

// HandleBulkSend triggers asynchronous ticket generation & queuing for filtered users.
func (h *Handler) HandleBulkSend(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req BulkSendTicketsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body to mean send all
		req = BulkSendTicketsRequest{}
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	resp, err := h.service.BulkSendTickets(r.Context(), req, claims.UserID, claims.Email, claims.Role, ip, ua)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to process bulk tickets: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, resp)
}

// HandleGetMyTicket returns the authenticated user's ticket with base64 QR code.
func (h *Handler) HandleGetMyTicket(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	resp, err := h.service.GetMyTicket(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, ErrTicketNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "your entry pass has not been issued yet")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to retrieve ticket: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, resp)
}

// HandleGetAnalytics returns live attendance metrics and charts.
func (h *Handler) HandleGetAnalytics(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.GetAnalytics(r.Context())
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to retrieve attendance analytics: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, resp)
}
