package competitions

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

const maxCompetitionBodyBytes = 16 << 10 // 16 KiB

// HandleCreateCompetition creates a new competition and assigns the caller as owner.
func (h *Handler) HandleCreateCompetition(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok || claims.UserID == "" {
		utils.ErrorJSON(w, http.StatusUnauthorized, "authentication required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCompetitionBodyBytes)
	var req CreateCompetitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	comp, err := h.service.CreateCompetition(r.Context(), req, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidSlug):
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrSlugAlreadyExists):
			utils.ErrorJSON(w, http.StatusConflict, "competition slug is already taken")
		case errors.Is(err, ErrUserAlreadyOwnsCompetition):
			utils.ErrorJSON(w, http.StatusConflict, "you may only be the owner of one competition at a time")
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to create competition")
		}
		return
	}

	utils.JSON(w, http.StatusCreated, comp)
}

// HandleGetCompetition returns public metadata for the resolved competition.
func (h *Handler) HandleGetCompetition(w http.ResponseWriter, r *http.Request) {
	comp, ok := GetCompetition(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusNotFound, "competition not found")
		return
	}

	resp := CompetitionResponse{
		Competition: *comp,
	}

	if compCtx, hasMembership := FromContext(r.Context()); hasMembership {
		resp.Role = &compCtx.Role
	}

	utils.JSON(w, http.StatusOK, resp)
}

// HandleUpdateCompetition updates competition name and description.
func (h *Handler) HandleUpdateCompetition(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCompetitionBodyBytes)
	var req UpdateCompetitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	comp, err := h.service.UpdateCompetition(r.Context(), compCtx.CompetitionID, req, compCtx.UserID)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to update competition")
		return
	}

	utils.JSON(w, http.StatusOK, comp)
}

// HandleTransferOwnership transfers ownership of the competition to another user.
func (h *Handler) HandleTransferOwnership(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCompetitionBodyBytes)
	var req TransferOwnershipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	err := h.service.TransferOwnership(r.Context(), compCtx.CompetitionID, compCtx.UserID, req.NewOwnerUserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTargetAlreadyOwnsCompetition):
			utils.ErrorJSON(w, http.StatusConflict, "target user is already an owner of another competition")
		case errors.Is(err, ErrMemberNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, "owner membership not found")
		default:
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "competition ownership transferred successfully",
	})
}

// HandleAssignMember assigns a role to a user in the competition.
func (h *Handler) HandleAssignMember(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCompetitionBodyBytes)
	var req AssignMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	err := h.service.AssignOrUpdateMemberRole(r.Context(), compCtx.CompetitionID, compCtx.UserID, compCtx.Role, req.UserID, req.Role)
	if err != nil {
		switch {
		case errors.Is(err, ErrPermissionDenied):
			utils.ErrorJSON(w, http.StatusForbidden, "admins cannot assign the admin role or modify existing admins")
		case errors.Is(err, ErrCannotModifyOwner):
			utils.ErrorJSON(w, http.StatusForbidden, err.Error())
		default:
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "member role assigned successfully",
	})
}

// HandleUpdateMemberRole modifies an existing member's role.
func (h *Handler) HandleUpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}

	targetUserID := chi.URLParam(r, "userId")
	if targetUserID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "user ID is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCompetitionBodyBytes)
	var req UpdateMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	err := h.service.AssignOrUpdateMemberRole(r.Context(), compCtx.CompetitionID, compCtx.UserID, compCtx.Role, targetUserID, req.Role)
	if err != nil {
		switch {
		case errors.Is(err, ErrPermissionDenied):
			utils.ErrorJSON(w, http.StatusForbidden, "admins cannot assign the admin role or modify existing admins")
		case errors.Is(err, ErrCannotSelfDemote):
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrCannotModifyOwner):
			utils.ErrorJSON(w, http.StatusForbidden, err.Error())
		default:
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "member role updated successfully",
	})
}

// HandleRemoveMember removes a user from the competition.
func (h *Handler) HandleRemoveMember(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}

	targetUserID := chi.URLParam(r, "userId")
	if targetUserID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "user ID is required")
		return
	}

	err := h.service.RemoveMember(r.Context(), compCtx.CompetitionID, compCtx.UserID, compCtx.Role, targetUserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrCannotRemoveOwner):
			utils.ErrorJSON(w, http.StatusForbidden, "cannot remove the competition owner; transfer ownership first")
		case errors.Is(err, ErrPermissionDenied):
			utils.ErrorJSON(w, http.StatusForbidden, "admins cannot remove other admins")
		case errors.Is(err, ErrMemberNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, "member not found in this competition")
		default:
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "member removed successfully",
	})
}

// HandleListMembers lists members in the competition.
func (h *Handler) HandleListMembers(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}

	roleFilter := r.URL.Query().Get("role")
	members, err := h.service.ListMembers(r.Context(), compCtx.CompetitionID, roleFilter)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to list members")
		return
	}

	utils.JSON(w, http.StatusOK, MemberListResponse{
		Members: members,
		Total:   len(members),
	})
}

// HandleGetMyMembership returns the calling user's membership and active role in this competition.
func (h *Handler) HandleGetMyMembership(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}

	utils.JSON(w, http.StatusOK, compCtx)
}
