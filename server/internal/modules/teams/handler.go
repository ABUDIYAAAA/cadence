package teams

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/internal/ratelimit"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/go-chi/chi/v5"
)

const (
	joinAttemptsPerMinute int64 = 10
)

type Handler struct {
	service *Service
	limiter *ratelimit.Limiter
}

// NewHandler builds the teams handler. limiter may be nil (join attempts are then not throttled).
func NewHandler(service *Service, limiter *ratelimit.Limiter) *Handler {
	return &Handler{service: service, limiter: limiter}
}

// writeError maps domain errors to HTTP responses.
func writeError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, ErrTeamNotFound):
		utils.ErrorJSON(w, http.StatusNotFound, "team not found")
	case errors.Is(err, ErrMemberNotFound), errors.Is(err, ErrUserNotFound):
		utils.ErrorJSON(w, http.StatusNotFound, "user is not a member of this team")
	case errors.Is(err, ErrInvalidCode):
		utils.ErrorJSON(w, http.StatusNotFound, "invalid join code")
	case errors.Is(err, ErrTeamAlreadyExists):
		utils.ErrorJSON(w, http.StatusConflict, "team with this name already exists")
	case errors.Is(err, ErrAlreadyInTeam):
		utils.ErrorJSON(w, http.StatusConflict, "user is already in a team")
	case errors.Is(err, ErrTeamFull):
		utils.ErrorJSON(w, http.StatusConflict, "team is full")
	case errors.Is(err, ErrTeamFrozen):
		utils.ErrorJSON(w, http.StatusConflict, "team changes are frozen")
	case errors.Is(err, ErrNotInTeam):
		utils.ErrorJSON(w, http.StatusConflict, "you are not in a team")
	case errors.Is(err, ErrNotLeader):
		utils.ErrorJSON(w, http.StatusForbidden, "only the team leader can do this")
	case errors.Is(err, ErrNotParticipant):
		utils.ErrorJSON(w, http.StatusBadRequest, "only participants can be on a team")
	case errors.Is(err, ErrLeaderNotMember), errors.Is(err, ErrCannotKickSelf):
		utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
	default:
		utils.ErrorJSON(w, http.StatusInternalServerError, fallback)
	}
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	if errs := utils.ValidateStruct(dst); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return false
	}
	return true
}

func callerID(w http.ResponseWriter, r *http.Request) (string, bool) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	return claims.UserID, true
}

// ---- participant flows ----

// HandleCreate lets a participant start a team and become its leader.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	var req CreateTeamRequest
	if !decode(w, r, &req) {
		return
	}
	team, err := h.service.CreateTeam(r.Context(), userID, req)
	if err != nil {
		writeError(w, err, "failed to create team")
		return
	}
	utils.JSON(w, http.StatusCreated, team)
}

// HandleJoin lets a participant join a team using its code.
func (h *Handler) HandleJoin(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	// Throttle guessing of join codes.
	if h.limiter != nil {
		res, err := h.limiter.Allow(r.Context(), fmt.Sprintf("ratelimit:team_join:%s", userID), joinAttemptsPerMinute, time.Minute)
		if err == nil && !res.Allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(res.ResetAfter.Seconds())))
			utils.ErrorJSON(w, http.StatusTooManyRequests, "too many join attempts, try again shortly")
			return
		}
	}
	var req JoinTeamRequest
	if !decode(w, r, &req) {
		return
	}
	team, err := h.service.JoinTeam(r.Context(), userID, req)
	if err != nil {
		writeError(w, err, "failed to join team")
		return
	}
	utils.JSON(w, http.StatusOK, team)
}

// HandleLeave removes the caller from their team.
func (h *Handler) HandleLeave(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	if err := h.service.LeaveTeam(r.Context(), userID); err != nil {
		writeError(w, err, "failed to leave team")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{"message": "you left the team"})
}

// HandleRenameOwn lets the leader rename their team.
func (h *Handler) HandleRenameOwn(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	var req RenameTeamRequest
	if !decode(w, r, &req) {
		return
	}
	team, err := h.service.RenameOwnTeam(r.Context(), userID, req)
	if err != nil {
		writeError(w, err, "failed to rename team")
		return
	}
	utils.JSON(w, http.StatusOK, team)
}

// HandleRegenerateOwnCode lets the leader replace the join code.
func (h *Handler) HandleRegenerateOwnCode(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	team, err := h.service.RegenerateOwnCode(r.Context(), userID)
	if err != nil {
		writeError(w, err, "failed to regenerate join code")
		return
	}
	utils.JSON(w, http.StatusOK, team)
}

// HandleKick lets the leader remove a teammate.
func (h *Handler) HandleKick(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	team, err := h.service.KickMember(r.Context(), userID, chi.URLParam(r, "userId"))
	if err != nil {
		writeError(w, err, "failed to remove member")
		return
	}
	utils.JSON(w, http.StatusOK, team)
}

// HandleMyTeam fetches the caller's team and teammates.
func (h *Handler) HandleMyTeam(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	team, err := h.service.GetMyTeam(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrTeamNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "you are not in a team yet")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to fetch team")
		return
	}
	utils.JSON(w, http.StatusOK, team)
}

// ---- reads ----

// HandleList returns paginated teams with member counts and leader details.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	limit, _ := strconv.Atoi(query.Get("limit"))

	filter := TeamFilter{Page: page, Limit: limit, Search: query.Get("search")}

	// Judges only see teams assigned to their panels.
	if claims, ok := auth.GetUserClaims(r.Context()); ok && claims.Role == "judge" {
		filter.JudgeID = &claims.UserID
	}

	resp, err := h.service.ListTeams(r.Context(), filter)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to list teams")
		return
	}
	utils.JSON(w, http.StatusOK, resp)
}

// HandleGet returns full team details with all member profiles.
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var team *TeamDetailResponse
	var err error
	if claims, ok := auth.GetUserClaims(r.Context()); ok && claims.Role == "judge" {
		team, err = h.service.GetTeamForJudge(r.Context(), id, claims.UserID)
	} else {
		team, err = h.service.GetTeamByID(r.Context(), id)
	}
	if err != nil {
		writeError(w, err, "failed to fetch team")
		return
	}
	utils.JSON(w, http.StatusOK, team)
}

// ---- staff flows ----

// HandleUpdate updates team properties (name, leader, max size).
func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	var req UpdateTeamRequest
	if !decode(w, r, &req) {
		return
	}
	team, err := h.service.UpdateTeam(r.Context(), chi.URLParam(r, "id"), req)
	if err != nil {
		writeError(w, err, "failed to update team")
		return
	}
	utils.JSON(w, http.StatusOK, team)
}

// HandleDelete deletes a team and detaches all members safely.
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteTeam(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, err, "failed to delete team")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "team deleted and members detached successfully",
	})
}

// HandleAddMember places a team-less participant into a team.
func (h *Handler) HandleAddMember(w http.ResponseWriter, r *http.Request) {
	var req AddMemberRequest
	if !decode(w, r, &req) {
		return
	}
	if err := h.service.AddMember(r.Context(), chi.URLParam(r, "id"), req.UserID); err != nil {
		writeError(w, err, "failed to add member")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{"message": "member added to team successfully"})
}

// HandleRemoveMember removes a user from a team.
func (h *Handler) HandleRemoveMember(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RemoveMember(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "userId")); err != nil {
		writeError(w, err, "failed to remove member from team")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{"message": "member removed from team successfully"})
}

// HandleRegenerateCode replaces a team's join code.
func (h *Handler) HandleRegenerateCode(w http.ResponseWriter, r *http.Request) {
	team, err := h.service.RegenerateCode(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, err, "failed to regenerate join code")
		return
	}
	utils.JSON(w, http.StatusOK, team)
}

func (h *Handler) HandleFreeze(w http.ResponseWriter, r *http.Request) {
	h.setFrozen(w, r, true)
}

func (h *Handler) HandleUnfreeze(w http.ResponseWriter, r *http.Request) {
	h.setFrozen(w, r, false)
}

func (h *Handler) setFrozen(w http.ResponseWriter, r *http.Request, frozen bool) {
	team, err := h.service.SetFrozen(r.Context(), chi.URLParam(r, "id"), frozen)
	if err != nil {
		writeError(w, err, "failed to update team freeze state")
		return
	}
	utils.JSON(w, http.StatusOK, team)
}

func (h *Handler) HandleFreezeAll(w http.ResponseWriter, r *http.Request) {
	h.setFrozenAll(w, r, true)
}

func (h *Handler) HandleUnfreezeAll(w http.ResponseWriter, r *http.Request) {
	h.setFrozenAll(w, r, false)
}

func (h *Handler) setFrozenAll(w http.ResponseWriter, r *http.Request, frozen bool) {
	settings, err := h.service.SetFrozenAll(r.Context(), frozen)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to update team freeze state")
		return
	}
	utils.JSON(w, http.StatusOK, settings)
}

func (h *Handler) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetSettings(r.Context())
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to load team settings")
		return
	}
	utils.JSON(w, http.StatusOK, settings)
}

func (h *Handler) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req UpdateSettingsRequest
	if !decode(w, r, &req) {
		return
	}
	settings, err := h.service.UpdateSettings(r.Context(), req)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to update team settings")
		return
	}
	utils.JSON(w, http.StatusOK, settings)
}
