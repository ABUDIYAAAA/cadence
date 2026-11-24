package users

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

func (r *Repository) CreateUserTx(
	ctx context.Context,
	tx pgx.Tx,
	email, name, role string,
	phone *string,
) (string, error) {
	if role != "dev" {
		role = "user"
	}

	query := `
		INSERT INTO users (email, name, phone_number, role, is_active)
		VALUES (LOWER($1), $2, $3, $4, TRUE)
		ON CONFLICT (email) DO UPDATE
		SET name = EXCLUDED.name,
		    phone_number = EXCLUDED.phone_number,
		    role = EXCLUDED.role,
		    is_active = TRUE,
		    deleted_at = NULL,
		    token_version = users.token_version + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE users.deleted_at IS NOT NULL
		RETURNING id
	`
	var id string
	err := tx.QueryRow(ctx, query, email, name, phone, role).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrUserAlreadyExists
		}
		return "", fmt.Errorf("insert user: %w", err)
	}

	return id, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*UserDetailResponse, error) {
	query := `
		SELECT id, email, name, phone_number, avatar_url, role, is_active, token_version, created_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`
	var u UserDetailResponse
	err := r.db.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.Name, &u.PhoneNumber, &u.AvatarURL, &u.Role, &u.IsActive, &u.TokenVersion, &u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*UserDetailResponse, error) {
	query := `
		SELECT id, email, name, phone_number, avatar_url, role, is_active, token_version, created_at
		FROM users
		WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL
	`
	var u UserDetailResponse
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.Name, &u.PhoneNumber, &u.AvatarURL, &u.Role, &u.IsActive, &u.TokenVersion, &u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

func (r *Repository) ListUsers(ctx context.Context, filter UserFilter) ([]UserDetailResponse, int64, error) {
	var conditions []string
	var args []any
	argIndex := 1

	if filter.Deleted {
		conditions = append(conditions, "deleted_at IS NOT NULL")
	} else {
		conditions = append(conditions, "deleted_at IS NULL")
	}

	if filter.Role != "" {
		conditions = append(conditions, fmt.Sprintf("role = $%d", argIndex))
		args = append(args, filter.Role)
		argIndex++
	}

	if filter.IsActive != nil {
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *filter.IsActive)
		argIndex++
	}

	if filter.Search != "" {
		searchTerm := "%" + strings.ToLower(filter.Search) + "%"
		conditions = append(conditions, fmt.Sprintf("(LOWER(name) LIKE $%d OR LOWER(email) LIKE $%d)", argIndex, argIndex))
		args = append(args, searchTerm)
		argIndex++
	}

	whereClause := strings.Join(conditions, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users WHERE %s", whereClause)
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

	query := fmt.Sprintf(`
		SELECT id, email, name, phone_number, avatar_url, role, is_active, created_at, deleted_at
		FROM users
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users query: %w", err)
	}
	defer rows.Close()

	usersList := make([]UserDetailResponse, 0, limit)
	for rows.Next() {
		var u UserDetailResponse
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Name, &u.PhoneNumber, &u.AvatarURL, &u.Role, &u.IsActive, &u.CreatedAt, &u.DeletedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan user row: %w", err)
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

	if req.IsActive != nil {
		sets = append(sets, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *req.IsActive)
		argIndex++
	}

	if req.Role != nil || req.IsActive != nil {
		sets = append(sets, "token_version = token_version + 1")
	}

	if len(sets) == 0 {
		return r.GetUserByID(ctx, id)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update tx: %w", err)
	}
	defer tx.Rollback(ctx)

	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d AND deleted_at IS NULL", strings.Join(sets, ", "), argIndex)
	args = append(args, id)

	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrUserNotFound
	}

	if req.Role != nil || (req.IsActive != nil && !*req.IsActive) {
		_, err = tx.Exec(ctx, `
			UPDATE refresh_tokens
			SET is_revoked = TRUE, revoked_at = CURRENT_TIMESTAMP
			WHERE user_id = $1 AND is_revoked = FALSE
		`, id)
		if err != nil {
			return nil, fmt.Errorf("revoke refresh tokens: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update tx: %w", err)
	}

	return r.GetUserByID(ctx, id)
}

func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_ = evictUserFromTeamTx(ctx, tx, id)

	tag, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return tx.Commit(ctx)
}

// RestoreUser reactivates a soft-deleted user. Returns ErrUserNotFound if the user is not deleted.
func (r *Repository) RestoreUser(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE users
		SET is_active = TRUE,
		    deleted_at = NULL,
		    token_version = token_version + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND deleted_at IS NOT NULL
	`, id)
	if err != nil {
		return fmt.Errorf("restore user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// EvictUserFromAnyTeam evicts a user from all teams they belong to across all competitions.
// If the user was the team leader, leadership transfers to the earliest-joined remaining member.
// If no members remain, the team is deleted.
func (r *Repository) EvictUserFromAnyTeam(ctx context.Context, userID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin evict team tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := evictUserFromTeamTx(ctx, tx, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func evictUserFromTeamTx(ctx context.Context, tx pgx.Tx, userID string) error {
	rows, err := tx.Query(ctx, `
		SELECT t.id, t.leader_id
		FROM team_members tm
		JOIN teams t ON t.id = tm.team_id
		WHERE tm.user_id = $1
		FOR UPDATE OF t
	`, userID)
	if err != nil {
		return err
	}
	type teamEntry struct {
		id       string
		leaderID *string
	}
	var affected []teamEntry
	for rows.Next() {
		var te teamEntry
		if err := rows.Scan(&te.id, &te.leaderID); err != nil {
			rows.Close()
			return err
		}
		affected = append(affected, te)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	_, _ = tx.Exec(ctx, `UPDATE users SET team_id = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, userID)

	for _, te := range affected {
		if _, err := tx.Exec(ctx, `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`, te.id, userID); err != nil {
			return err
		}

		var remaining int
		err = tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM team_members tm
			JOIN users u ON u.id = tm.user_id
			WHERE tm.team_id = $1 AND u.deleted_at IS NULL AND tm.user_id <> $2
		`, te.id, userID).Scan(&remaining)
		if err != nil {
			return err
		}

		if remaining == 0 {
			if _, err := tx.Exec(ctx, `DELETE FROM teams WHERE id = $1`, te.id); err != nil {
				return err
			}
		} else if te.leaderID != nil && *te.leaderID == userID {
			_, err = tx.Exec(ctx, `
				UPDATE teams SET updated_at = CURRENT_TIMESTAMP, leader_id = (
					SELECT tm.user_id FROM team_members tm
					JOIN users u ON u.id = tm.user_id
					WHERE tm.team_id = $1 AND u.deleted_at IS NULL AND tm.user_id <> $2
					ORDER BY tm.joined_at ASC, tm.user_id ASC LIMIT 1
				) WHERE id = $1`, te.id, userID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
