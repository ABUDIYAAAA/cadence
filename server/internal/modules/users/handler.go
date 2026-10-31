package users

import (
	"encoding/json"
	"errors"
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

// HandleBulkCreate imports multiple users. They set their own password through "forgot password".
func (h *Handler) HandleBulkCreate(w http.ResponseWriter, r *http.Request) {
	var req BulkCreateUsersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	resp, err := h.service.BulkCreateUsers(r.Context(), req)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.JSON(w, http.StatusCreated, resp)
}

// HandleCreate creates a single user.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	user, err := h.service.CreateUser(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrRoleNotPermitted) {
			utils.ErrorJSON(w, http.StatusForbidden, "volunteers can only add participants")
			return
		}
		if errors.Is(err, ErrUserAlreadyExists) {
			utils.ErrorJSON(w, http.StatusConflict, "user with this email already exists")
			return
		}
		utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.JSON(w, http.StatusCreated, user)
}

// HandleList returns paginated users with optional role, team, active, and search filters.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	if claims, ok := auth.GetUserClaims(r.Context()); ok && claims.Role != "admin" && claims.Role != "volunteer" {
		utils.ErrorJSON(w, http.StatusForbidden, "insufficient permissions to list users")
		return
	}

	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	limit, _ := strconv.Atoi(query.Get("limit"))

	filter := UserFilter{
		Page:   page,
		Limit:  limit,
		Role:   query.Get("role"),
		TeamID: query.Get("team_id"),
		Search: query.Get("search"),
		NoTeam: query.Get("no_team") == "true" || query.Get("no_team") == "1",
	}

	switch cs := query.Get("checkin_status"); cs {
	case "checked_in", "not_checked_in", "no_pass":
		filter.CheckinStatus = cs
	case "":
	default:
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid checkin_status")
		return
	}

	if activeParam := query.Get("is_active"); activeParam != "" {
		activeBool := activeParam == "true" || activeParam == "1"
		filter.IsActive = &activeBool
	}

	resp, err := h.service.ListUsers(r.Context(), filter)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	utils.JSON(w, http.StatusOK, resp)
}

// HandleGet returns a single user's details.
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	if claims, ok := auth.GetUserClaims(r.Context()); ok && claims.Role != "admin" && claims.Role != "volunteer" {
		utils.ErrorJSON(w, http.StatusForbidden, "insufficient permissions to view user details")
		return
	}

	id := chi.URLParam(r, "id")
	user, err := h.service.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "user not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to fetch user")
		return
	}

	utils.JSON(w, http.StatusOK, user)
}

// HandleUpdate updates editable fields on a user record.
func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	user, err := h.service.UpdateUser(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "user not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	utils.JSON(w, http.StatusOK, user)
}

// HandleDelete instantly removes and deactivates a user with <1ms Redis access token blocklist.
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteUser(r.Context(), id); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "user not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to remove user")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "user removed and sessions invalidated instantly",
	})
}

// HandleTriggerResetPassword triggers a password recovery email initiated by an administrator.
func (h *Handler) HandleTriggerResetPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.TriggerResetPassword(r.Context(), id); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "user not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to trigger password reset")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "password reset email triggered successfully",
	})
}
