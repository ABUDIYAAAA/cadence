package teams

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTeamNotFound      = errors.New("team not found")
	ErrTeamAlreadyExists = errors.New("team with this name already exists")
	ErrMemberNotFound    = errors.New("user is not a member of this team")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateTeam(ctx context.Context, name string, leaderID *string, memberIDs []string) (*TeamDetailResponse, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create team tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var teamID string
	insertQuery := `INSERT INTO teams (name, leader_id) VALUES ($1, $2) RETURNING id`
	if err := tx.QueryRow(ctx, insertQuery, strings.TrimSpace(name), leaderID).Scan(&teamID); err != nil {
		if strings.Contains(err.Error(), "teams_name_key") {
			return nil, ErrTeamAlreadyExists
		}
		return nil, fmt.Errorf("insert team: %w", err)
	}

	// Link leader to team
	if leaderID != nil && *leaderID != "" {
		_, err := tx.Exec(ctx, "UPDATE users SET team_id = $1 WHERE id = $2 AND deleted_at IS NULL", teamID, *leaderID)
		if err != nil {
			return nil, fmt.Errorf("link leader to team: %w", err)
		}
	}

	// Link initial members
	for _, mID := range memberIDs {
		if leaderID != nil && mID == *leaderID {
			continue
		}
		_, err := tx.Exec(ctx, "UPDATE users SET team_id = $1 WHERE id = $2 AND deleted_at IS NULL", teamID, mID)
		if err != nil {
			return nil, fmt.Errorf("link member %s to team: %w", mID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create team tx: %w", err)
	}

	return r.GetTeamByID(ctx, teamID)
}

func (r *Repository) GetTeamByID(ctx context.Context, id string) (*TeamDetailResponse, error) {
	query := `
		SELECT t.id, t.name, t.leader_id, t.created_at, t.updated_at
		FROM teams t
		WHERE t.id = $1
	`
	var resp TeamDetailResponse
	var leaderID *string
	err := r.db.QueryRow(ctx, query, id).Scan(
		&resp.ID, &resp.Name, &leaderID, &resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("get team by id: %w", err)
	}

	membersQuery := `
		SELECT id, email, name, phone_number, role, is_password_configured
		FROM users
		WHERE team_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, membersQuery, id)
	if err != nil {
		return nil, fmt.Errorf("fetch team members: %w", err)
	}
	defer rows.Close()

	resp.Members = make([]TeamMemberSummary, 0)
	for rows.Next() {
		var m TeamMemberSummary
		if err := rows.Scan(&m.ID, &m.Email, &m.Name, &m.PhoneNumber, &m.Role, &m.IsPasswordConfigured); err != nil {
			return nil, fmt.Errorf("scan member row: %w", err)
		}
		if leaderID != nil && m.ID == *leaderID {
			m.IsLeader = true
			resp.Leader = &m
		}
		resp.Members = append(resp.Members, m)
	}

	return &resp, nil
}

func (r *Repository) GetTeamByUserID(ctx context.Context, userID string) (*TeamDetailResponse, error) {
	var teamID *string
	err := r.db.QueryRow(ctx, "SELECT team_id FROM users WHERE id = $1 AND deleted_at IS NULL", userID).Scan(&teamID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("query user team: %w", err)
	}
	if teamID == nil {
		return nil, ErrTeamNotFound
	}

	return r.GetTeamByID(ctx, *teamID)
}

func (r *Repository) ListTeams(ctx context.Context, filter TeamFilter) ([]TeamResponse, int64, error) {
	var conditions []string
	var args []any
	argIndex := 1

	if filter.Search != "" {
		searchTerm := "%" + strings.ToLower(filter.Search) + "%"
		conditions = append(conditions, fmt.Sprintf("LOWER(t.name) LIKE $%d", argIndex))
		args = append(args, searchTerm)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM teams t %s", whereClause)
	var totalCount int64
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count teams: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	query := fmt.Sprintf(`
		SELECT t.id, t.name, t.leader_id, u.name, u.email,
		       COUNT(m.id) AS member_count, t.created_at, t.updated_at
		FROM teams t
		LEFT JOIN users u ON t.leader_id = u.id AND u.deleted_at IS NULL
		LEFT JOIN users m ON t.id = m.team_id AND m.deleted_at IS NULL
		%s
		GROUP BY t.id, t.name, t.leader_id, u.name, u.email, t.created_at, t.updated_at
		ORDER BY t.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list teams query: %w", err)
	}
	defer rows.Close()

	teamsList := make([]TeamResponse, 0, limit)
	for rows.Next() {
		var t TeamResponse
		if err := rows.Scan(
			&t.ID, &t.Name, &t.LeaderID, &t.LeaderName, &t.LeaderEmail,
			&t.MemberCount, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan team row: %w", err)
		}
		teamsList = append(teamsList, t)
	}

	return teamsList, totalCount, nil
}

func (r *Repository) UpdateTeam(ctx context.Context, id string, req UpdateTeamRequest) (*TeamDetailResponse, error) {
	var sets []string
	var args []any
	argIndex := 1

	if req.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, strings.TrimSpace(*req.Name))
		argIndex++
	}

	if req.LeaderID != nil {
		sets = append(sets, fmt.Sprintf("leader_id = $%d", argIndex))
		args = append(args, *req.LeaderID)
		argIndex++
	}

	if len(sets) == 0 {
		return r.GetTeamByID(ctx, id)
	}

	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	query := fmt.Sprintf("UPDATE teams SET %s WHERE id = $%d", strings.Join(sets, ", "), argIndex)
	args = append(args, id)

	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "teams_name_key") {
			return nil, ErrTeamAlreadyExists
		}
		return nil, fmt.Errorf("update team: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrTeamNotFound
	}

	// If leader changed, ensure leader is assigned to this team
	if req.LeaderID != nil && *req.LeaderID != "" {
		_, _ = r.db.Exec(ctx, "UPDATE users SET team_id = $1 WHERE id = $2 AND deleted_at IS NULL", id, *req.LeaderID)
	}

	return r.GetTeamByID(ctx, id)
}

func (r *Repository) DeleteTeam(ctx context.Context, id string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete team tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Detach all members safely
	_, err = tx.Exec(ctx, "UPDATE users SET team_id = NULL WHERE team_id = $1", id)
	if err != nil {
		return fmt.Errorf("detach team members: %w", err)
	}

	// Delete team
	tag, err := tx.Exec(ctx, "DELETE FROM teams WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete team: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTeamNotFound
	}

	return tx.Commit(ctx)
}

func (r *Repository) AddMember(ctx context.Context, teamID, userID string) error {
	var exists bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM teams WHERE id = $1)", teamID).Scan(&exists)
	if err != nil || !exists {
		return ErrTeamNotFound
	}

	tag, err := r.db.Exec(ctx, "UPDATE users SET team_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2 AND deleted_at IS NULL", teamID, userID)
	if err != nil {
		return fmt.Errorf("add member to team: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}

func (r *Repository) RemoveMember(ctx context.Context, teamID, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin remove member tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, "UPDATE users SET team_id = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = $1 AND team_id = $2", userID, teamID)
	if err != nil {
		return fmt.Errorf("remove member from team: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}

	// If this member was team leader, unset leader_id
	_, err = tx.Exec(ctx, "UPDATE teams SET leader_id = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = $1 AND leader_id = $2", teamID, userID)
	if err != nil {
		return fmt.Errorf("clear team leader on member removal: %w", err)
	}

	return tx.Commit(ctx)
}
