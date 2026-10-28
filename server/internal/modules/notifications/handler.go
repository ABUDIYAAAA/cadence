package notifications

import (
	"encoding/json"
	"net/http"
	"strconv"

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

// HandleGetVAPIDPublicKey returns the server's public key for Web Push registration.
func (h *Handler) HandleGetVAPIDPublicKey(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, map[string]string{
		"public_key": h.service.GetVAPIDPublicKey(),
	})
}

// HandleSubscribe saves a client's Web Push subscription.
func (h *Handler) HandleSubscribe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(&req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	ua := r.UserAgent()
	ip := r.RemoteAddr

	if err := h.service.Subscribe(r.Context(), claims.UserID, req, &ua, ip); err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to register push subscription: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "Push subscription registered successfully",
	})
}

// HandleUnsubscribe removes a client's Web Push subscription.
func (h *Handler) HandleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req UnsubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(&req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	ua := r.UserAgent()
	ip := r.RemoteAddr

	if err := h.service.Unsubscribe(r.Context(), claims.UserID, req, &ua, ip); err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to remove push subscription: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "Push subscription removed successfully",
	})
}

// HandleGetInbox returns paginated notifications and unread badge count for the authenticated user.
func (h *Handler) HandleGetInbox(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	resp, err := h.service.GetInbox(r.Context(), claims.UserID, page, limit)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to fetch inbox: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, resp)
}

// HandleMarkAsRead marks a single notification as read.
func (h *Handler) HandleMarkAsRead(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	notificationID := chi.URLParam(r, "id")
	if notificationID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "notification ID is required")
		return
	}

	if err := h.service.MarkAsRead(r.Context(), claims.UserID, notificationID); err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to mark as read: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message":         "Notification marked as read",
		"notification_id": notificationID,
	})
}

// HandleMarkAllAsRead marks all user notifications as read.
func (h *Handler) HandleMarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.MarkAllAsRead(r.Context(), claims.UserID); err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to mark all as read: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "All notifications marked as read",
	})
}

// HandleBroadcast publishes a notification to targets and enqueues Web Push dispatches.
func (h *Handler) HandleBroadcast(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req BroadcastNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(&req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	ua := r.UserAgent()
	ip := r.RemoteAddr

	resp, err := h.service.Broadcast(r.Context(), req, claims.UserID, claims.Email, claims.Role, ip, ua)
	if err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "failed to broadcast notification: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusCreated, resp)
}

// HandleGetAdminHistory lists sent notifications with recipient and read stats.
func (h *Handler) HandleGetAdminHistory(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	resp, err := h.service.GetAdminHistory(r.Context(), page, limit)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to fetch notification history: "+err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, resp)
}
