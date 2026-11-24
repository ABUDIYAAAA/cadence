package competitions

import (
	"encoding/json"
	"errors"
	"net/http"

	"devclubxnst.com/cadence/internal/modules/events"
	"devclubxnst.com/cadence/pkg/utils"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

const maxCompetitionBodyBytes = 16 << 10 // 16 KiB

// HandleCreateCompetition creates a new competition inside the active event.
// Accessible to event owners, admins, and global devs.
func (h *Handler) HandleCreateCompetition(w http.ResponseWriter, r *http.Request) {
	evCtx, ok := events.FromContext(r.Context())
	if !ok || evCtx.EventID == "" {
		utils.ErrorJSON(w, http.StatusForbidden, "event context missing")
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

	comp, err := h.service.CreateCompetition(r.Context(), evCtx.EventID, req, evCtx.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidSlug):
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrSlugAlreadyExists):
			utils.ErrorJSON(w, http.StatusConflict, "competition slug is already taken in this event")
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to create competition")
		}
		return
	}

	utils.JSON(w, http.StatusCreated, comp)
}

// HandleListCompetitions lists all competitions in the active event.
func (h *Handler) HandleListCompetitions(w http.ResponseWriter, r *http.Request) {
	var eventID string
	if evCtx, ok := events.FromContext(r.Context()); ok && evCtx != nil {
		eventID = evCtx.EventID
	} else if ev, ok := events.GetEvent(r.Context()); ok && ev != nil {
		eventID = ev.ID
	}
	if eventID == "" {
		utils.ErrorJSON(w, http.StatusForbidden, "event context missing")
		return
	}

	comps, err := h.service.ListCompetitionsByEvent(r.Context(), eventID)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to list competitions")
		return
	}

	utils.JSON(w, http.StatusOK, CompetitionListResponse{
		Competitions: comps,
		Total:        len(comps),
	})
}

// HandleGetCompetition returns metadata for the resolved competition.
func (h *Handler) HandleGetCompetition(w http.ResponseWriter, r *http.Request) {
	comp, ok := GetCompetition(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusNotFound, "competition not found")
		return
	}

	resp := CompetitionResponse{
		Competition: *comp,
	}

	if compCtx, hasCtx := FromContext(r.Context()); hasCtx && compCtx.Role != "" {
		resp.Role = &compCtx.Role
	} else if evCtx, hasEv := events.FromContext(r.Context()); hasEv && evCtx.Role != "" {
		resp.Role = &evCtx.Role
	}

	utils.JSON(w, http.StatusOK, resp)
}

// HandleUpdateCompetition updates competition name, description, or slug.
// Accessible to event owners, admins, and global devs.
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

	comp, err := h.service.UpdateCompetition(r.Context(), compCtx.EventID, compCtx.CompetitionID, compCtx.Slug, req, compCtx.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidSlug):
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrSlugAlreadyExists):
			utils.ErrorJSON(w, http.StatusConflict, "competition slug is already taken in this event")
		case errors.Is(err, ErrCompetitionNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, "competition not found")
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to update competition")
		}
		return
	}

	utils.JSON(w, http.StatusOK, comp)
}

// HandleDeleteCompetition deletes a competition from the active event.
// Accessible to event owners, admins, and global devs.
func (h *Handler) HandleDeleteCompetition(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}

	if err := h.service.DeleteCompetition(r.Context(), compCtx.EventID, compCtx.CompetitionID, compCtx.UserID); err != nil {
		if errors.Is(err, ErrCompetitionNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, "competition not found")
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to delete competition")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]string{
		"message": "competition deleted successfully",
	})
}
