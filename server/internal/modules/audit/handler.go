package audit

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"devclubxnst.com/cadence/pkg/utils"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// HandleList returns a paginated list of audit logs with multi-field filtering.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	filter := h.parseFilter(r)
	list, totalCount, err := h.repo.ListAuditLogs(r.Context(), filter)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to query audit logs")
		return
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}

	totalPages := int((totalCount + int64(limit) - 1) / int64(limit))

	utils.JSON(w, http.StatusOK, PaginatedAuditLogsResponse{
		Data:       list,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	})
}

// HandleExport streams a CSV download of the filtered audit trail.
func (h *Handler) HandleExport(w http.ResponseWriter, r *http.Request) {
	filter := h.parseFilter(r)
	filter.Limit = 5000 // Upper limit for direct CSV export
	filter.Page = 1

	list, _, err := h.repo.ListAuditLogs(r.Context(), filter)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to export audit logs")
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"audit_logs_%s.csv\"", time.Now().Format("20060102_150405")))

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header row
	_ = writer.Write([]string{
		"ID",
		"Timestamp",
		"Action",
		"Actor ID",
		"Actor Email",
		"Actor Role",
		"Target Type",
		"Target ID",
		"Description",
		"IP Address",
		"Device ID",
	})

	for _, item := range list {
		actorID := ""
		if item.ActorID != nil {
			actorID = *item.ActorID
		}
		actorEmail := ""
		if item.ActorEmail != nil {
			actorEmail = *item.ActorEmail
		}
		actorRole := ""
		if item.ActorRole != nil {
			actorRole = *item.ActorRole
		}
		targetID := ""
		if item.TargetID != nil {
			targetID = *item.TargetID
		}
		ip := ""
		if item.IPAddress != nil {
			ip = *item.IPAddress
		}
		devID := ""
		if item.DeviceID != nil {
			devID = *item.DeviceID
		}

		_ = writer.Write([]string{
			item.ID,
			item.CreatedAt.Format(time.RFC3339),
			item.Action,
			actorID,
			actorEmail,
			actorRole,
			item.TargetType,
			targetID,
			item.Description,
			ip,
			devID,
		})
	}
}

func (h *Handler) parseFilter(r *http.Request) AuditLogFilter {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))

	filter := AuditLogFilter{
		Page:       page,
		Limit:      limit,
		Action:     q.Get("action"),
		ActorID:    q.Get("actor_id"),
		TargetType: q.Get("target_type"),
		TargetID:   q.Get("target_id"),
		Search:     q.Get("search"),
	}

	if fromStr := q.Get("from_date"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			filter.FromDate = &t
		}
	}

	if toStr := q.Get("to_date"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			filter.ToDate = &t
		}
	}

	return filter
}
