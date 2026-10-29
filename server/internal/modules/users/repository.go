package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user with this email already exists")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

func (r *Repository) GetOrCreateTeamByNameTx(ctx context.Context, tx pgx.Tx, name string) (string, error) {
	cleanName := strings.TrimSpace(name)
	var teamID string
	err := tx.QueryRow(ctx, "SELECT id FROM teams WHERE LOWER(name) = LOWER($1)", cleanName).Scan(&teamID)
	if err == nil {
		return teamID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("lookup team by name: %w", err)
	}

	insertErr := tx.QueryRow(ctx, "INSERT INTO teams (name) VALUES ($1) RETURNING id", cleanName).Scan(&teamID)
	if insertErr != nil {
		return "", fmt.Errorf("create team: %w", insertErr)
	}
	return teamID, nil
}

func (r *Repository) SetTeamLeaderTx(ctx context.Context, tx pgx.Tx, teamID, leaderID string) error {
	_, err := tx.Exec(ctx, "UPDATE teams SET leader_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2", leaderID, teamID)
	if err != nil {
		return fmt.Errorf("set team leader: %w", err)
	}
	return nil
}

func (r *Repository) CreateUserTx(
	ctx context.Context,
	tx pgx.Tx,
	email, name, role string,
	phone *string,
	teamID *string,
) (string, error) {
	query := `
		INSERT INTO users (email, name, role, phone_number, team_id, is_password_configured, is_active)
		VALUES (LOWER($1), $2, $3, $4, $5, FALSE, TRUE)
		RETURNING id
	`
	var id string
	err := tx.QueryRow(ctx, query, email, name, role, phone, teamID).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "users_email_key") {
			return "", ErrUserAlreadyExists
		}
		return "", fmt.Errorf("insert user: %w", err)
	}
	return id, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*UserDetailResponse, error) {
	query := `
		SELECT u.id, u.email, u.name, u.phone_number, u.role, u.team_id, t.name,
		       (t.leader_id = u.id) AS is_leader, u.is_password_configured, u.is_active, u.created_at
		FROM users u
		LEFT JOIN teams t ON u.team_id = t.id
		WHERE u.id = $1 AND u.deleted_at IS NULL
	`
	var u UserDetailResponse
	var teamName *string
	var isLeader *bool
	err := r.db.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.Name, &u.PhoneNumber, &u.Role, &u.TeamID, &teamName,
		&isLeader, &u.IsPasswordConfigured, &u.IsActive, &u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	u.TeamName = teamName
	if isLeader != nil {
		u.IsLeader = *isLeader
	}
	return &u, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*UserDetailResponse, error) {
	query := `
		SELECT u.id, u.email, u.name, u.phone_number, u.role, u.team_id, t.name,
		       (t.leader_id = u.id) AS is_leader, u.is_password_configured, u.is_active, u.created_at
		FROM users u
		LEFT JOIN teams t ON u.team_id = t.id
		WHERE LOWER(u.email) = LOWER($1) AND u.deleted_at IS NULL
	`
	var u UserDetailResponse
	var teamName *string
	var isLeader *bool
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.Name, &u.PhoneNumber, &u.Role, &u.TeamID, &teamName,
		&isLeader, &u.IsPasswordConfigured, &u.IsActive, &u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	u.TeamName = teamName
	if isLeader != nil {
		u.IsLeader = *isLeader
	}
	return &u, nil
}

func (r *Repository) ListUsers(ctx context.Context, filter UserFilter) ([]UserDetailResponse, int64, error) {
	var conditions []string
	var args []any
	argIndex := 1

	conditions = append(conditions, "u.deleted_at IS NULL")

	if filter.Role != "" {
		conditions = append(conditions, fmt.Sprintf("u.role = $%d", argIndex))
		args = append(args, filter.Role)
		argIndex++
	}

	if filter.TeamID != "" {
		conditions = append(conditions, fmt.Sprintf("u.team_id = $%d", argIndex))
		args = append(args, filter.TeamID)
		argIndex++
	}

	if filter.IsActive != nil {
		conditions = append(conditions, fmt.Sprintf("u.is_active = $%d", argIndex))
		args = append(args, *filter.IsActive)
		argIndex++
	}

	if filter.Search != "" {
		searchTerm := "%" + strings.ToLower(filter.Search) + "%"
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

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users u LEFT JOIN tickets tk ON tk.user_id = u.id WHERE %s", whereClause)
	var totalCount int64
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
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

	// Newest arrivals first when looking at who has checked in
	orderBy := "u.created_at DESC"
	if filter.CheckinStatus == "checked_in" {
		orderBy = "tk.checked_in_at DESC NULLS LAST, u.created_at DESC"
	}

	query := fmt.Sprintf(`
		SELECT u.id, u.email, u.name, u.phone_number, u.role, u.team_id, t.name,
		       (t.leader_id = u.id) AS is_leader, u.is_password_configured, u.is_active, u.created_at,
		       CASE WHEN tk.id IS NULL THEN 'no_pass' WHEN tk.is_checked_in THEN 'checked_in' ELSE 'not_checked_in' END,
		       tk.checked_in_at
		FROM users u
		LEFT JOIN teams t ON u.team_id = t.id
		LEFT JOIN tickets tk ON tk.user_id = u.id
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users query: %w", err)
	}
	defer rows.Close()

	usersList := make([]UserDetailResponse, 0, limit)
	for rows.Next() {
		var u UserDetailResponse
		var teamName *string
		var isLeader *bool
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Name, &u.PhoneNumber, &u.Role, &u.TeamID, &teamName,
			&isLeader, &u.IsPasswordConfigured, &u.IsActive, &u.CreatedAt,
			&u.CheckinStatus, &u.CheckedInAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan user row: %w", err)
		}
		u.TeamName = teamName
		if isLeader != nil {
			u.IsLeader = *isLeader
		}
		usersList = append(usersList, u)
	}

	return usersList, totalCount, nil
}

func (r *Repository) UpdateUser(ctx context.Context, id string, req UpdateUserRequest) (*UserDetailResponse, error) {
	var sets []string
	var args []any
	argIndex := 1

	if req.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, *req.Name)
		argIndex++
	}

	if req.PhoneNumber != nil {
		sets = append(sets, fmt.Sprintf("phone_number = $%d", argIndex))
		args = append(args, *req.PhoneNumber)
		argIndex++
	}

	if req.Role != nil {
		sets = append(sets, fmt.Sprintf("role = $%d", argIndex))
		args = append(args, *req.Role)
		argIndex++
	}

	if req.TeamID != nil {
		sets = append(sets, fmt.Sprintf("team_id = $%d", argIndex))
		args = append(args, *req.TeamID)
		argIndex++
	}

	if req.IsActive != nil {
		sets = append(sets, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *req.IsActive)
		argIndex++
	}

	if len(sets) == 0 {
		return r.GetUserByID(ctx, id)
	}

	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d AND deleted_at IS NULL", strings.Join(sets, ", "), argIndex)
	args = append(args, id)

	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrUserNotFound
	}

	return r.GetUserByID(ctx, id)
}

func (r *Repository) SoftDeleteUser(ctx context.Context, id string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Clear leadership if this user is a team leader
	_, err = tx.Exec(ctx, "UPDATE teams SET leader_id = NULL, updated_at = CURRENT_TIMESTAMP WHERE leader_id = $1", id)
	if err != nil {
		return fmt.Errorf("clear team leadership: %w", err)
	}

	// Soft-delete user and detach from team
	delQuery := `
		UPDATE users
		SET is_active = FALSE,
		    deleted_at = CURRENT_TIMESTAMP,
		    team_id = NULL,
		    token_version = token_version + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND deleted_at IS NULL
	`
	tag, err := tx.Exec(ctx, delQuery, id)
	if err != nil {
		return fmt.Errorf("soft delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	// Revoke all refresh tokens
	_, err = tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET is_revoked = TRUE, revoked_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND is_revoked = FALSE
	`, id)
	if err != nil {
		return fmt.Errorf("revoke refresh tokens on delete: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *Repository) CreateResetOrSetupToken(ctx context.Context, userID, tokenHash, purpose string, expiresAt time.Time) error {
	query := `
		INSERT INTO password_reset_tokens (user_id, token_hash, purpose, expires_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.db.Exec(ctx, query, userID, tokenHash, purpose, expiresAt)
	if err != nil {
		return fmt.Errorf("insert reset/setup token: %w", err)
	}
	return nil
}

func (r *Repository) GetUninvitedUserIDs(ctx context.Context) ([]string, error) {
	query := `
		SELECT id
		FROM users
		WHERE is_password_configured = FALSE AND is_active = TRUE AND deleted_at IS NULL
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query uninvited user ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
