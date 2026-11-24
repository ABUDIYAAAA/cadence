package events

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

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

const (
	maxEventBodyBytes = 16 << 10 // 16 KiB
	maxBulkBodyBytes  = 4 << 20  // 4 MiB
)

// HandleCreateEvent creates a new event and assigns the designated user or caller as owner.
// Strictly restricted to users with global 'dev' role.
func (h *Handler) HandleCreateEvent(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok || claims.UserID == "" {
		utils.ErrorJSON(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if claims.GlobalRole != "dev" {
		utils.ErrorJSON(w, http.StatusForbidden, "developer access required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxEventBodyBytes)
	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	ev, err := h.service.CreateEvent(r.Context(), req, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidSlug):
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrSlugAlreadyExists):
			utils.ErrorJSON(w, http.StatusConflict, "event slug is already taken")
		case errors.Is(err, ErrUserAlreadyOwnsEvent):
			utils.ErrorJSON(w, http.StatusConflict, "event owner already owns another event")
		case strings.Contains(err.Error(), "owner user not found"):
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to create event")
		}
		return
	}

	utils.JSON(w, http.StatusCreated, ev)
}

// HandleDeleteEvent deletes an event. Strictly restricted to global 'dev' role.
func (h *Handler) HandleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok || claims.UserID == "" {
		utils.ErrorJSON(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if claims.GlobalRole != "dev" {
		utils.ErrorJSON(w, http.StatusForbidden, "developer access required")
		return
	}

	var eventID string
	if ev, hasEv := GetEvent(r.Context()); hasEv && ev != nil {
		eventID = ev.ID
	} else if slug := chi.URLParam(r, "eventSlug"); slug != "" {
		ev, err := h.service.GetEventBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, ErrEventNotFound) {
				utils.ErrorJSON(w, http.StatusNotFound, "event not found")
				return
			}
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to resolve event")
			return
		}
		eventID = ev.ID
	} else if id := chi.URLParam(r, "eventId"); id != "" {
		eventID = id
	}

	if eventID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "event identifier is required")
		return
	}

	if err := h.service.DeleteEvent(r.Context(), eventID, claims.UserID); err != nil {
		if errors.Is(err, ErrEventNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "event not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to delete event")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "event deleted successfully",
	})
}

// HandleListEvents returns all events.
func (h *Handler) HandleListEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.service.ListEvents(r.Context())
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to list events")
		return
	}
	utils.JSON(w, http.StatusOK, events)
}

// HandleGetEvent returns public metadata for the resolved event.
func (h *Handler) HandleGetEvent(w http.ResponseWriter, r *http.Request) {
	ev, ok := GetEvent(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusNotFound, "event not found")
		return
	}

	resp := EventResponse{
		Event: *ev,
	}

	if evCtx, hasMembership := FromContext(r.Context()); hasMembership {
		resp.Role = &evCtx.Role
	}

	utils.JSON(w, http.StatusOK, resp)
}

// HandleUpdateEvent updates event name and description.
func (h *Handler) HandleUpdateEvent(w http.ResponseWriter, r *http.Request) {
	evCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "event context missing")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxEventBodyBytes)
	var req UpdateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	ev, err := h.service.UpdateEvent(r.Context(), evCtx.EventID, req, evCtx.UserID)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to update event")
		return
	}

	utils.JSON(w, http.StatusOK, ev)
}

// HandleTransferOwnership transfers ownership of the event to another user.
func (h *Handler) HandleTransferOwnership(w http.ResponseWriter, r *http.Request) {
	evCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "event context missing")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxEventBodyBytes)
	var req TransferOwnershipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	if req.NewOwnerUserID == "" && (req.NewOwnerEmail == nil || *req.NewOwnerEmail == "") {
		utils.ErrorJSON(w, http.StatusBadRequest, "either new_owner_user_id or new_owner_email must be provided")
		return
	}

	err := h.service.TransferOwnership(r.Context(), evCtx.EventID, evCtx.UserID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrTargetAlreadyOwnsEvent):
			utils.ErrorJSON(w, http.StatusConflict, "target user is already an owner of another event")
		case errors.Is(err, ErrMemberNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, "owner membership not found")
		default:
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "event ownership transferred successfully",
	})
}

// HandleAddEventUser adds a single user to an event by email, creating a background account if needed.
func (h *Handler) HandleAddEventUser(w http.ResponseWriter, r *http.Request) {
	evCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "event context missing")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxEventBodyBytes)
	var req AddEventUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	// Volunteers can only add participants
	if evCtx.Role == "volunteer" {
		if req.Role != "" && req.Role != "participant" {
			utils.ErrorJSON(w, http.StatusForbidden, "volunteers can only add participants")
			return
		}
		req.Role = "participant"
	}

	user, _, err := h.service.AddEventUser(r.Context(), evCtx.EventID, evCtx.UserID, evCtx.Role, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrPermissionDenied):
			utils.ErrorJSON(w, http.StatusForbidden, "admins cannot assign the admin role or modify existing admins")
		case errors.Is(err, ErrCannotModifyOwner):
			utils.ErrorJSON(w, http.StatusForbidden, err.Error())
		default:
			if strings.Contains(err.Error(), "banned") {
				utils.ErrorJSON(w, http.StatusForbidden, err.Error())
				return
			}
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	utils.JSON(w, http.StatusCreated, user)
}

// HandleBulkAddEventUsers adds multiple users to an event in bulk by email.
func (h *Handler) HandleBulkAddEventUsers(w http.ResponseWriter, r *http.Request) {
	evCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "event context missing")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBulkBodyBytes)
	var req BulkAddEventUsersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	resp, err := h.service.BulkAddEventUsers(r.Context(), evCtx.EventID, evCtx.UserID, evCtx.Role, req)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.JSON(w, http.StatusCreated, resp)
}

// HandleAssignMember assigns a role to a user in the event.
func (h *Handler) HandleAssignMember(w http.ResponseWriter, r *http.Request) {
	evCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "event context missing")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxEventBodyBytes)
	var req AssignMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	if (req.UserID == nil || *req.UserID == "") && (req.Email == nil || *req.Email == "") {
		utils.ErrorJSON(w, http.StatusBadRequest, "either user_id or email must be provided")
		return
	}

	if req.Email != nil && *req.Email != "" {
		user, _, err := h.service.AddEventUser(r.Context(), evCtx.EventID, evCtx.UserID, evCtx.Role, AddEventUserRequest{
			Email: *req.Email,
			Name:  req.Name,
			Role:  req.Role,
		})
		if err != nil {
			switch {
			case errors.Is(err, ErrPermissionDenied):
				utils.ErrorJSON(w, http.StatusForbidden, "admins cannot assign the admin role or modify existing admins")
			case errors.Is(err, ErrCannotModifyOwner):
				utils.ErrorJSON(w, http.StatusForbidden, err.Error())
			default:
				if strings.Contains(err.Error(), "banned") {
					utils.ErrorJSON(w, http.StatusForbidden, err.Error())
					return
				}
				utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		utils.JSON(w, http.StatusOK, user)
		return
	}

	err := h.service.AssignOrUpdateMemberRole(r.Context(), evCtx.EventID, evCtx.UserID, evCtx.Role, *req.UserID, req.Role)
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

// HandleUpdateMemberRole modifies an existing member's role in the event.
func (h *Handler) HandleUpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	evCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "event context missing")
		return
	}

	targetUserID := chi.URLParam(r, "userId")
	if targetUserID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "user ID is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxEventBodyBytes)
	var req UpdateMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	err := h.service.AssignOrUpdateMemberRole(r.Context(), evCtx.EventID, evCtx.UserID, evCtx.Role, targetUserID, req.Role)
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

// HandleRemoveMember removes a user from the event.
func (h *Handler) HandleRemoveMember(w http.ResponseWriter, r *http.Request) {
	evCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "event context missing")
		return
	}

	targetUserID := chi.URLParam(r, "userId")
	if targetUserID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "user ID is required")
		return
	}

	err := h.service.RemoveMember(r.Context(), evCtx.EventID, evCtx.UserID, evCtx.Role, targetUserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrCannotRemoveOwner):
			utils.ErrorJSON(w, http.StatusForbidden, "cannot remove the event owner; transfer ownership first")
		case errors.Is(err, ErrPermissionDenied):
			utils.ErrorJSON(w, http.StatusForbidden, "admins cannot remove other admins")
		case errors.Is(err, ErrMemberNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, "member not found in this event")
		default:
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "member removed successfully",
	})
}

// HandleListMembers lists members in the event.
func (h *Handler) HandleListMembers(w http.ResponseWriter, r *http.Request) {
	evCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "event context missing")
		return
	}

	roleFilter := r.URL.Query().Get("role")
	members, err := h.service.ListMembers(r.Context(), evCtx.EventID, roleFilter)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to list members")
		return
	}

	utils.JSON(w, http.StatusOK, MemberListResponse{
		Members: members,
		Total:   len(members),
	})
}

// HandleGetMyMembership returns the calling user's membership and active role in this event.
func (h *Handler) HandleGetMyMembership(w http.ResponseWriter, r *http.Request) {
	evCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "event context missing")
		return
	}

	utils.JSON(w, http.StatusOK, evCtx)
}

// HandleBanMember bans a user from the event.
func (h *Handler) HandleBanMember(w http.ResponseWriter, r *http.Request) {
	evCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusInternalServerError, "event context missing")
		return
	}

	targetUserID := chi.URLParam(r, "userId")
	if targetUserID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "target user id is required")
		return
	}

	var req BanMemberRequest
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	err := h.service.BanMember(r.Context(), evCtx.EventID, evCtx.UserID, evCtx.Role, targetUserID, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, ErrMemberNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, "member not found in this event")
		case errors.Is(err, ErrPermissionDenied):
			utils.ErrorJSON(w, http.StatusForbidden, "admins cannot ban other admins")
		case strings.Contains(err.Error(), "cannot ban the event owner"):
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to ban member")
		}
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{"message": "member successfully banned from event"})
}

// HandleUnbanMember restores a banned user's access in the event.
func (h *Handler) HandleUnbanMember(w http.ResponseWriter, r *http.Request) {
	evCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusInternalServerError, "event context missing")
		return
	}

	targetUserID := chi.URLParam(r, "userId")
	if targetUserID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "target user id is required")
		return
	}

	err := h.service.UnbanMember(r.Context(), evCtx.EventID, evCtx.UserID, targetUserID)
	if err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "member not found in this event")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to unban member")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{"message": "member successfully unbanned in event"})
}

// HandleListEventUsers lists users scoped to this event.
func (h *Handler) HandleListEventUsers(w http.ResponseWriter, r *http.Request) {
	evCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusInternalServerError, "event context missing")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	filter := EventUserFilter{
		Page:          page,
		Limit:         limit,
		Role:          r.URL.Query().Get("role"),
		CompetitionID: r.URL.Query().Get("competition_id"),
		TeamID:        r.URL.Query().Get("team_id"),
		NoTeam:        r.URL.Query().Get("no_team") == "true" || r.URL.Query().Get("no_team") == "1",
		Search:        r.URL.Query().Get("search"),
		CheckinStatus: r.URL.Query().Get("checkin_status"),
	}

	if bannedStr := r.URL.Query().Get("is_banned"); bannedStr != "" {
		isBanned := bannedStr == "true"
		filter.IsBanned = &isBanned
	}

	resp, err := h.service.ListEventUsers(r.Context(), evCtx.EventID, filter)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to list event users")
		return
	}

	utils.JSON(w, http.StatusOK, resp)
}
