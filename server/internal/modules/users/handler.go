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

// HandleBulkCreate imports multiple platform users (restricted to devs).
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

// HandleCreate creates a single platform user (restricted to devs).
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
		if errors.Is(err, ErrUserAlreadyExists) {
			utils.ErrorJSON(w, http.StatusConflict, "user with this email already exists")
			return
		}
		utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.JSON(w, http.StatusCreated, user)
}

// HandleList returns paginated platform users with optional role, active, deleted, and search filters.
// Restricted strictly to devs.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	if claims, ok := auth.GetUserClaims(r.Context()); !ok || claims.Role != "dev" {
		utils.ErrorJSON(w, http.StatusForbidden, "developer access required")
		return
	}

	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	limit, _ := strconv.Atoi(query.Get("limit"))

	filter := UserFilter{
		Page:    page,
		Limit:   limit,
		Role:    query.Get("role"),
		Search:  query.Get("search"),
		Deleted: query.Get("deleted") == "true" || query.Get("deleted") == "1",
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

// HandleGet returns a single platform user's details.
// Restricted strictly to devs.
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	if claims, ok := auth.GetUserClaims(r.Context()); !ok || claims.Role != "dev" {
		utils.ErrorJSON(w, http.StatusForbidden, "developer access required")
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

// HandleRestore reactivates a removed user.
func (h *Handler) HandleRestore(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.RestoreUser(r.Context(), id); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "removed user not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to reactivate user")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "user reactivated",
	})
}

// HandleBlacklist globally blacklists a user (developer access only).
func (h *Handler) HandleBlacklist(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.BlacklistUser(r.Context(), id); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "user not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to blacklist user")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "user blacklisted globally and sessions invalidated",
	})
}

// HandleUnblacklist removes a global blacklist from a user (developer access only).
func (h *Handler) HandleUnblacklist(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.UnblacklistUser(r.Context(), id); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "user not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to unblacklist user")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "user global blacklist lifted",
	})
}
