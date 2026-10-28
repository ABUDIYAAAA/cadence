package teams

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

// HandleCreate creates a new team with an optional leader and initial members.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	team, err := h.service.CreateTeam(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrTeamAlreadyExists) {
			utils.ErrorJSON(w, http.StatusConflict, "team with this name already exists")
			return
		}
		utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.JSON(w, http.StatusCreated, team)
}

// HandleList returns paginated teams with member counts and leader details.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	limit, _ := strconv.Atoi(query.Get("limit"))

	filter := TeamFilter{
		Page:   page,
		Limit:  limit,
		Search: query.Get("search"),
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
	team, err := h.service.GetTeamByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrTeamNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "team not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to fetch team")
		return
	}

	utils.JSON(w, http.StatusOK, team)
}

// HandleMyTeam allows an authenticated user to fetch their assigned team and teammates.
func (h *Handler) HandleMyTeam(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserClaims(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	team, err := h.service.GetMyTeam(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, ErrTeamNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "you have not been assigned to a team yet")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to fetch team")
		return
	}

	utils.JSON(w, http.StatusOK, team)
}

// HandleUpdate updates team properties (name, leader).
func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	team, err := h.service.UpdateTeam(r.Context(), id, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrTeamNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, "team not found")
		case errors.Is(err, ErrTeamAlreadyExists):
			utils.ErrorJSON(w, http.StatusConflict, "team with this name already exists")
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to update team")
		}
		return
	}

	utils.JSON(w, http.StatusOK, team)
}

// HandleDelete deletes a team and detaches all members safely.
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteTeam(r.Context(), id); err != nil {
		if errors.Is(err, ErrTeamNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "team not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to delete team")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "team deleted and members detached successfully",
	})
}

// HandleAddMember adds an existing user to a team.
func (h *Handler) HandleAddMember(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	var req AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := utils.ValidateStruct(req); len(errs) > 0 {
		utils.ValidationErrorJSON(w, errs)
		return
	}

	if err := h.service.AddMember(r.Context(), teamID, req.UserID); err != nil {
		if errors.Is(err, ErrTeamNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "team not found")
			return
		}
		utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "member added to team successfully",
	})
}

// HandleRemoveMember removes a user from a team.
func (h *Handler) HandleRemoveMember(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")

	if err := h.service.RemoveMember(r.Context(), teamID, userID); err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "user is not a member of this team")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to remove member from team")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "member removed from team successfully",
	})
}
