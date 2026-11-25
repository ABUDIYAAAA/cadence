package competitions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"devclubxnst.com/cadence/internal/modules/events"
	"devclubxnst.com/cadence/pkg/audit"
	"devclubxnst.com/cadence/pkg/logger"
	"devclubxnst.com/cadence/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

var (
	ErrMemberNotFound    = errors.New("member not found in this competition")
	ErrPermissionDenied  = errors.New("insufficient permissions for this member action")
	ErrCannotModifyOwner = errors.New("cannot modify the event owner")
)

// ListCompetitionUsers returns paginated members enrolled in compID with their team in this competition and event ticket check-in status.
func (r *Repository) ListCompetitionUsers(ctx context.Context, eventID, compID string, filter events.EventUserFilter) (*events.EventUserListResponse, error) {
	conditions := []string{"cm.competition_id = $1", "u.deleted_at IS NULL"}
	args := []any{compID}
	argIndex := 2

	if filter.Role != "" {
		conditions = append(conditions, fmt.Sprintf("cm.role::text = $%d", argIndex))
		args = append(args, filter.Role)
		argIndex++
	}
	if filter.IsBanned != nil {
		conditions = append(conditions, fmt.Sprintf("cm.is_banned = $%d", argIndex))
		args = append(args, *filter.IsBanned)
		argIndex++
	}
	if filter.TeamID != "" {
		conditions = append(conditions, fmt.Sprintf("tm.team_id = $%d", argIndex))
		args = append(args, filter.TeamID)
		argIndex++
	}
	if filter.NoTeam {
		conditions = append(conditions, "tm.team_id IS NULL")
	}
	if filter.Search != "" {
		searchTerm := "%" + strings.ToLower(strings.TrimSpace(filter.Search)) + "%"
		conditions = append(conditions, fmt.Sprintf("(LOWER(u.name) LIKE $%d OR LOWER(u.email) LIKE $%d)", argIndex, argIndex))
		args = append(args, searchTerm)
		argIndex++
	}
	switch filter.CheckinStatus {
	case "checked_in":
		conditions = append(conditions, "tk.is_checked_in = true")
	case "not_checked_in":
		conditions = append(conditions, "tk.id IS NOT NULL AND tk.is_checked_in = false")
	case "no_pass":
		conditions = append(conditions, "tk.id IS NULL")
	}

	whereClause := strings.Join(conditions, " AND ")
	eventArgIdx := argIndex
	args = append(args, eventID)
	argIndex++

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM competition_members cm
		JOIN users u ON cm.user_id = u.id
		LEFT JOIN team_members tm ON tm.user_id = u.id AND tm.competition_id = cm.competition_id
		LEFT JOIN tickets tk ON tk.user_id = u.id AND tk.event_id = $%d
		WHERE %s
	`, eventArgIdx, whereClause)

	var total int64
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count competition users: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 25
	} else if limit > 100 {
		limit = 100
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	query := fmt.Sprintf(`
		SELECT u.id, u.email, u.name, u.phone_number, u.avatar_url,
		       cm.role::text, cm.is_banned, cm.banned_at,
		       t.id, t.name, (t.leader_id = u.id),
		       cm.created_at,
		       CASE WHEN tk.id IS NULL THEN 'no_pass' WHEN tk.is_checked_in THEN 'checked_in' ELSE 'not_checked_in' END,
		       tk.checked_in_at
		FROM competition_members cm
		JOIN users u ON cm.user_id = u.id
		LEFT JOIN team_members tm ON tm.user_id = u.id AND tm.competition_id = cm.competition_id
		LEFT JOIN teams t ON t.id = tm.team_id
		LEFT JOIN tickets tk ON tk.user_id = u.id AND tk.event_id = $%d
		WHERE %s
		ORDER BY cm.created_at DESC
		LIMIT $%d OFFSET $%d
	`, eventArgIdx, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list competition users: %w", err)
	}
	defer rows.Close()

	list := make([]events.EventUserDetailResponse, 0)
	for rows.Next() {
		var u events.EventUserDetailResponse
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Name, &u.PhoneNumber, &u.AvatarURL,
			&u.Role, &u.IsBanned, &u.BannedAt,
			&u.TeamID, &u.TeamName, &u.IsLeader,
			&u.CreatedAt, &u.CheckinStatus, &u.CheckedInAt,
		); err != nil {
			return nil, fmt.Errorf("scan competition user: %w", err)
		}
		list = append(list, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &events.EventUserListResponse{
		Users: list,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

// AddCompetitionUser upserts a user account, ensures parent event membership, and enrolls them in compID.
func (r *Repository) AddCompetitionUser(
	ctx context.Context,
	eventID, compID, actorRole, email string,
	name, phone *string,
	role string,
) (*events.EventUserDetailResponse, bool, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, false, errors.New("email cannot be empty")
	}
	if role == "" {
		role = "participant"
	}
	if role == "owner" {
		return nil, false, errors.New("cannot assign owner role in competition")
	}
	if actorRole == "volunteer" && role != "participant" {
		return nil, false, ErrPermissionDenied
	}
	if actorRole == "admin" && (role == "admin" || role == "owner") {
		return nil, false, ErrPermissionDenied
	}

	userName := ""
	if name != nil {
		userName = strings.TrimSpace(*name)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Provision or restore user account through users.Provisioner.
	provisioned, err := r.users.ProvisionByEmailTx(ctx, tx, email, userName, phone)
	if err != nil {
		return nil, false, fmt.Errorf("upsert user: %w", err)
	}
	targetUserID := provisioned.ID
	createdAccount := provisioned.Created

	// 2. Ensure parent event membership exists (do not downgrade existing event role)
	var evBanned bool
	err = tx.QueryRow(ctx, `SELECT is_banned FROM user_events WHERE event_id = $1 AND user_id = $2`, eventID, targetUserID).Scan(&evBanned)
	if err == nil {
		if evBanned {
			return nil, false, errors.New("user is banned from this event; unban in event first")
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_events (event_id, user_id, role)
			VALUES ($1, $2, $3::event_role)
			ON CONFLICT (event_id, user_id) DO NOTHING
		`, eventID, targetUserID, role); err != nil {
			return nil, false, fmt.Errorf("ensure event membership: %w", err)
		}
	} else {
		return nil, false, fmt.Errorf("check event membership: %w", err)
	}

	// 3. Upsert competition membership with role hierarchy checks
	var existingRole string
	var isBanned bool
	err = tx.QueryRow(ctx, `
		SELECT role::text, is_banned
		FROM competition_members
		WHERE competition_id = $1 AND user_id = $2
		FOR UPDATE
	`, compID, targetUserID).Scan(&existingRole, &isBanned)
	if err == nil {
		if isBanned {
			return nil, false, errors.New("user is banned from this competition; unban first")
		}
		if existingRole == "owner" && role != "owner" {
			return nil, false, ErrCannotModifyOwner
		}
		if actorRole == "admin" && (existingRole == "admin" || existingRole == "owner") {
			return nil, false, ErrPermissionDenied
		}
		if _, err := tx.Exec(ctx, `
			UPDATE competition_members
			SET role = $1::event_role, updated_at = CURRENT_TIMESTAMP
			WHERE competition_id = $2 AND user_id = $3
		`, role, compID, targetUserID); err != nil {
			return nil, false, fmt.Errorf("update competition member role: %w", err)
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO competition_members (competition_id, user_id, role)
			VALUES ($1, $2, $3::event_role)
		`, compID, targetUserID, role); err != nil {
			return nil, false, fmt.Errorf("insert competition member: %w", err)
		}
	} else {
		return nil, false, fmt.Errorf("check competition membership: %w", err)
	}

	// 4. Fetch populated detail
	var detail events.EventUserDetailResponse
	err = tx.QueryRow(ctx, `
		SELECT u.id, u.email, u.name, u.phone_number, u.avatar_url,
		       cm.role::text, cm.is_banned, cm.banned_at,
		       t.id, t.name, (t.leader_id = u.id),
		       cm.created_at,
		       CASE WHEN tk.id IS NULL THEN 'no_pass' WHEN tk.is_checked_in THEN 'checked_in' ELSE 'not_checked_in' END,
		       tk.checked_in_at
		FROM competition_members cm
		JOIN users u ON cm.user_id = u.id
		LEFT JOIN team_members tm ON tm.user_id = u.id AND tm.competition_id = cm.competition_id
		LEFT JOIN teams t ON t.id = tm.team_id
		LEFT JOIN tickets tk ON tk.user_id = u.id AND tk.event_id = $1
		WHERE cm.competition_id = $2 AND u.id = $3
	`, eventID, compID, targetUserID).Scan(
		&detail.ID, &detail.Email, &detail.Name, &detail.PhoneNumber, &detail.AvatarURL,
		&detail.Role, &detail.IsBanned, &detail.BannedAt,
		&detail.TeamID, &detail.TeamName, &detail.IsLeader,
		&detail.CreatedAt, &detail.CheckinStatus, &detail.CheckedInAt,
	)
	if err != nil {
		return nil, false, fmt.Errorf("fetch competition user detail: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit tx: %w", err)
	}
	r.users.SyncProvisionedUserRedis(ctx, provisioned)
	return &detail, createdAccount, nil
}

// AssignUserToTeamInCompetition delegates named team formation/assignment inside compID to teams.RosterManager.
func (r *Repository) AssignUserToTeamInCompetition(ctx context.Context, compID, actorID, userID, teamName string) (teamCreated bool, assigned bool, err error) {
	if r.teams == nil {
		return false, false, errors.New("team roster manager not configured")
	}
	_, _, teamCreated, assigned, err = r.teams.AssignUserToNamedTeam(ctx, compID, actorID, userID, teamName)
	return teamCreated, assigned, err
}

// UpdateCompetitionMemberRole updates a member's role in compID.
func (r *Repository) UpdateCompetitionMemberRole(ctx context.Context, compID, actorRole, targetUserID, role string) error {
	if role == "owner" {
		return errors.New("cannot assign owner role in competition")
	}
	if actorRole == "admin" && role == "admin" {
		return ErrPermissionDenied
	}

	var currentRole string
	err := r.db.QueryRow(ctx, `SELECT role::text FROM competition_members WHERE competition_id = $1 AND user_id = $2`, compID, targetUserID).Scan(&currentRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return fmt.Errorf("find competition member: %w", err)
	}
	if currentRole == "owner" {
		return ErrCannotModifyOwner
	}
	if actorRole == "admin" && currentRole == "admin" {
		return ErrPermissionDenied
	}

	_, err = r.db.Exec(ctx, `
		UPDATE competition_members
		SET role = $1::event_role, updated_at = CURRENT_TIMESTAMP
		WHERE competition_id = $2 AND user_id = $3
	`, role, compID, targetUserID)
	return err
}

// RemoveCompetitionMember removes a user from compID and evicts them from any team in compID.
func (r *Repository) RemoveCompetitionMember(ctx context.Context, compID, actorRole, targetUserID string) error {
	var currentRole string
	err := r.db.QueryRow(ctx, `SELECT role::text FROM competition_members WHERE competition_id = $1 AND user_id = $2`, compID, targetUserID).Scan(&currentRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return fmt.Errorf("find competition member: %w", err)
	}
	if currentRole == "owner" {
		return ErrCannotModifyOwner
	}
	if actorRole == "admin" && currentRole == "admin" {
		return ErrPermissionDenied
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var auditEviction func()
	if r.teams != nil {
		auditEviction, err = r.teams.EvictAndAuditCompetitionTeamTx(ctx, tx, compID, targetUserID, r.auditor)
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM competition_members WHERE competition_id = $1 AND user_id = $2`, compID, targetUserID); err != nil {
		return fmt.Errorf("delete competition member: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if auditEviction != nil {
		auditEviction()
	}
	return nil
}

// BanCompetitionMember bans a member in compID and evicts them from any team in compID.
func (r *Repository) BanCompetitionMember(ctx context.Context, compID, actorRole, targetUserID string) error {
	var currentRole string
	err := r.db.QueryRow(ctx, `SELECT role::text FROM competition_members WHERE competition_id = $1 AND user_id = $2`, compID, targetUserID).Scan(&currentRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return fmt.Errorf("find competition member to ban: %w", err)
	}
	if currentRole == "owner" {
		return ErrCannotModifyOwner
	}
	if actorRole == "admin" && currentRole == "admin" {
		return ErrPermissionDenied
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE competition_members
		SET is_banned = TRUE, banned_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE competition_id = $1 AND user_id = $2
	`, compID, targetUserID); err != nil {
		return fmt.Errorf("ban competition member: %w", err)
	}
	var auditEviction func()
	if r.teams != nil {
		auditEviction, err = r.teams.EvictAndAuditCompetitionTeamTx(ctx, tx, compID, targetUserID, r.auditor)
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if auditEviction != nil {
		auditEviction()
	}
	return nil
}

// GetMemberRole returns the user's explicit competition membership role and ban flag.
// Returns ErrMemberNotFound when the user has no explicit row in competition_members.
func (r *Repository) GetMemberRole(ctx context.Context, compID, userID string) (string, bool, error) {
	var role string
	var isBanned bool
	err := r.db.QueryRow(ctx, `
		SELECT role::text, is_banned
		FROM competition_members
		WHERE competition_id = $1 AND user_id = $2
	`, compID, userID).Scan(&role, &isBanned)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, ErrMemberNotFound
		}
		return "", false, fmt.Errorf("get competition member role: %w", err)
	}
	return role, isBanned, nil
}

// UnbanCompetitionMember clears the ban flag on a competition member.
func (r *Repository) UnbanCompetitionMember(ctx context.Context, compID, targetUserID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE competition_members
		SET is_banned = FALSE, banned_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE competition_id = $1 AND user_id = $2
	`, compID, targetUserID)
	if err != nil {
		return fmt.Errorf("unban competition member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}

// --- Service layer ---

func (s *Service) ListCompetitionUsers(ctx context.Context, eventID, compID string, filter events.EventUserFilter) (*events.EventUserListResponse, error) {
	return s.repo.ListCompetitionUsers(ctx, eventID, compID, filter)
}

func (s *Service) AddCompetitionUser(ctx context.Context, eventID, compID, actorID, actorRole string, req events.AddEventUserRequest) (*events.EventUserDetailResponse, error) {
	detail, created, err := s.repo.AddCompetitionUser(ctx, eventID, compID, actorRole, req.Email, req.Name, req.PhoneNumber, req.Role)
	if err != nil {
		return nil, err
	}
	if req.TeamName != nil && strings.TrimSpace(*req.TeamName) != "" && detail.Role == "participant" {
		if _, _, teamErr := s.repo.AssignUserToTeamInCompetition(ctx, compID, actorID, detail.ID, *req.TeamName); teamErr != nil {
			logger.FromContext(ctx).Warn("failed to assign added competition member to team",
				slog.String("competition_id", compID),
				slog.String("user_id", detail.ID),
				slog.Any("error", teamErr),
			)
		}
	}
	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionCompetitionUpdate, audit.TargetCompetition, &compID, fmt.Sprintf("Added %s to competition as %s", detail.Email, detail.Role), map[string]any{
			"target_user_id":  detail.ID,
			"role":            detail.Role,
			"created_account": created,
		})
	}
	return detail, nil
}

func (s *Service) BulkAddCompetitionUsers(ctx context.Context, eventID, compID, actorID, actorRole string, req events.BulkAddEventUsersRequest) (*events.BulkAddEventUsersResponse, error) {
	resp := &events.BulkAddEventUsersResponse{
		TotalProcessed: len(req.Users),
		Users:          make([]events.EventUserDetailResponse, 0, len(req.Users)),
	}

	for _, u := range req.Users {
		detail, created, err := s.repo.AddCompetitionUser(ctx, eventID, compID, actorRole, u.Email, u.Name, u.PhoneNumber, u.Role)
		if err != nil {
			resp.SkippedCount++
			resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("%s: %s", u.Email, err.Error()))
			continue
		}
		if created {
			resp.TotalCreated++
		} else {
			resp.TotalAdded++
		}
		if u.TeamName != nil && strings.TrimSpace(*u.TeamName) != "" {
			if detail.Role != "participant" {
				resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("%s: team assignment skipped (role is %s)", u.Email, detail.Role))
			} else {
				teamCreated, assigned, teamErr := s.repo.AssignUserToTeamInCompetition(ctx, compID, actorID, detail.ID, *u.TeamName)
				if teamErr != nil {
					resp.SkippedErrors = append(resp.SkippedErrors, fmt.Sprintf("%s: %s", u.Email, teamErr.Error()))
				} else {
					if teamCreated {
						resp.TeamsCreated++
					}
					if assigned {
						resp.TeamsAssigned++
					}
				}
			}
		}
		resp.Users = append(resp.Users, *detail)
	}

	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionCompetitionUpdate, audit.TargetCompetition, &compID, fmt.Sprintf("Bulk imported %d competition members", resp.TotalCreated+resp.TotalAdded), map[string]any{
			"total_processed": resp.TotalProcessed,
			"total_created":   resp.TotalCreated,
			"total_added":     resp.TotalAdded,
			"skipped_count":   resp.SkippedCount,
		})
	}
	return resp, nil
}

func (s *Service) UpdateCompetitionMemberRole(ctx context.Context, eventID, compID, actorID, actorRole, targetUserID, role string) error {
	if err := s.repo.UpdateCompetitionMemberRole(ctx, compID, actorRole, targetUserID, role); err != nil {
		return err
	}
	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionCompetitionUpdate, audit.TargetCompetition, &compID, fmt.Sprintf("Updated competition member role to %s", role), map[string]any{
			"target_user_id": targetUserID,
			"role":           role,
		})
	}
	return nil
}

func (s *Service) RemoveCompetitionMember(ctx context.Context, eventID, compID, actorID, actorRole, targetUserID string) error {
	if err := s.repo.RemoveCompetitionMember(ctx, compID, actorRole, targetUserID); err != nil {
		return err
	}
	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionCompetitionUpdate, audit.TargetCompetition, &compID, "Removed member from competition", map[string]any{
			"target_user_id": targetUserID,
		})
	}
	return nil
}

func (s *Service) BanCompetitionMember(ctx context.Context, eventID, compID, actorID, actorRole, targetUserID string, reason *string) error {
	if err := s.repo.BanCompetitionMember(ctx, compID, actorRole, targetUserID); err != nil {
		return err
	}
	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionCompetitionUpdate, audit.TargetCompetition, &compID, "Banned member from competition", map[string]any{
			"target_user_id": targetUserID,
			"reason":         reason,
			"banned_at":      time.Now().UTC(),
		})
	}
	return nil
}

func (s *Service) UnbanCompetitionMember(ctx context.Context, eventID, compID, actorID, targetUserID string) error {
	if err := s.repo.UnbanCompetitionMember(ctx, compID, targetUserID); err != nil {
		return err
	}
	if s.auditor != nil {
		s.auditor.RecordAction(ctx, audit.ActionCompetitionUpdate, audit.TargetCompetition, &compID, "Unbanned member in competition", map[string]any{
			"target_user_id": targetUserID,
		})
	}
	return nil
}

// --- HTTP Handlers ---

func (h *Handler) HandleListCompetitionUsers(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	filter := events.EventUserFilter{
		Page:          page,
		Limit:         limit,
		Role:          r.URL.Query().Get("role"),
		TeamID:        r.URL.Query().Get("team_id"),
		NoTeam:        r.URL.Query().Get("no_team") == "true" || r.URL.Query().Get("no_team") == "1",
		Search:        r.URL.Query().Get("search"),
		CheckinStatus: r.URL.Query().Get("checkin_status"),
	}
	if bannedStr := r.URL.Query().Get("is_banned"); bannedStr != "" {
		isBanned := bannedStr == "true"
		filter.IsBanned = &isBanned
	}

	resp, err := h.service.ListCompetitionUsers(r.Context(), compCtx.EventID, compCtx.CompetitionID, filter)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to list competition members")
		return
	}
	utils.JSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleAddCompetitionUser(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}

	var req events.AddEventUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}
	if validationErrs := utils.ValidateStruct(req); validationErrs != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"error": "validation failed", "details": validationErrs})
		return
	}

	detail, err := h.service.AddCompetitionUser(r.Context(), compCtx.EventID, compCtx.CompetitionID, compCtx.UserID, compCtx.Role, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrPermissionDenied):
			utils.ErrorJSON(w, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrCannotModifyOwner), strings.Contains(err.Error(), "banned"):
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to add competition member")
		}
		return
	}
	utils.JSON(w, http.StatusCreated, detail)
}

func (h *Handler) HandleBulkAddCompetitionUsers(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}

	var req events.BulkAddEventUsersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}
	if validationErrs := utils.ValidateStruct(req); validationErrs != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"error": "validation failed", "details": validationErrs})
		return
	}

	resp, err := h.service.BulkAddCompetitionUsers(r.Context(), compCtx.EventID, compCtx.CompetitionID, compCtx.UserID, compCtx.Role, req)
	if err != nil {
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to bulk import competition members")
		return
	}
	utils.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) HandleUpdateCompetitionMemberRole(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}
	targetUserID := chi.URLParam(r, "userId")
	if targetUserID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "target user id is required")
		return
	}

	var req events.UpdateMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorJSON(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}
	if validationErrs := utils.ValidateStruct(req); validationErrs != nil {
		utils.JSON(w, http.StatusBadRequest, map[string]any{"error": "validation failed", "details": validationErrs})
		return
	}

	err := h.service.UpdateCompetitionMemberRole(r.Context(), compCtx.EventID, compCtx.CompetitionID, compCtx.UserID, compCtx.Role, targetUserID, req.Role)
	if err != nil {
		switch {
		case errors.Is(err, ErrMemberNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrPermissionDenied):
			utils.ErrorJSON(w, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrCannotModifyOwner):
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to update competition member role")
		}
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{"message": "competition member role updated"})
}

func (h *Handler) HandleRemoveCompetitionMember(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}
	targetUserID := chi.URLParam(r, "userId")
	if targetUserID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "target user id is required")
		return
	}

	err := h.service.RemoveCompetitionMember(r.Context(), compCtx.EventID, compCtx.CompetitionID, compCtx.UserID, compCtx.Role, targetUserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrMemberNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrPermissionDenied):
			utils.ErrorJSON(w, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrCannotModifyOwner):
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to remove competition member")
		}
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{"message": "member removed from competition"})
}

func (h *Handler) HandleBanCompetitionMember(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}
	targetUserID := chi.URLParam(r, "userId")
	if targetUserID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "target user id is required")
		return
	}

	var req events.BanMemberRequest
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	err := h.service.BanCompetitionMember(r.Context(), compCtx.EventID, compCtx.CompetitionID, compCtx.UserID, compCtx.Role, targetUserID, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, ErrMemberNotFound):
			utils.ErrorJSON(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrPermissionDenied):
			utils.ErrorJSON(w, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrCannotModifyOwner):
			utils.ErrorJSON(w, http.StatusBadRequest, err.Error())
		default:
			utils.ErrorJSON(w, http.StatusInternalServerError, "failed to ban competition member")
		}
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{"message": "member banned from competition"})
}

func (h *Handler) HandleUnbanCompetitionMember(w http.ResponseWriter, r *http.Request) {
	compCtx, ok := FromContext(r.Context())
	if !ok {
		utils.ErrorJSON(w, http.StatusForbidden, "competition context missing")
		return
	}
	targetUserID := chi.URLParam(r, "userId")
	if targetUserID == "" {
		utils.ErrorJSON(w, http.StatusBadRequest, "target user id is required")
		return
	}

	err := h.service.UnbanCompetitionMember(r.Context(), compCtx.EventID, compCtx.CompetitionID, compCtx.UserID, targetUserID)
	if err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			utils.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}
		utils.ErrorJSON(w, http.StatusInternalServerError, "failed to unban competition member")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{"message": "member unbanned in competition"})
}
