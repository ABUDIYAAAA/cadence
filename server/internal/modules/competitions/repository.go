package competitions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrCompetitionNotFound         = errors.New("competition not found")
	ErrSlugAlreadyExists           = errors.New("competition slug already in use")
	ErrUserAlreadyOwnsCompetition  = errors.New("user already owns a competition")
	ErrTargetAlreadyOwnsCompetition = errors.New("target user already owns another competition")
	ErrCannotRemoveOwner           = errors.New("cannot remove competition owner; transfer ownership first")
	ErrMemberNotFound              = errors.New("competition member not found")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateCompetition inserts a new competition and binds the creator as 'owner' within a single transaction.
// A user is strictly prevented from owning more than one competition simultaneously.
func (r *Repository) CreateCompetition(ctx context.Context, slug, name string, description *string, ownerUserID string) (*Competition, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check if user is already an owner of any competition
	var existingCompID string
	checkOwnerQuery := `SELECT competition_id FROM user_competitions WHERE user_id = $1 AND role = 'owner' LIMIT 1`
	err = tx.QueryRow(ctx, checkOwnerQuery, ownerUserID).Scan(&existingCompID)
	if err == nil {
		return nil, ErrUserAlreadyOwnsCompetition
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("check existing ownership: %w", err)
	}

	insertComp := `
		INSERT INTO competitions (slug, name, description)
		VALUES (LOWER($1), $2, $3)
		RETURNING id, slug, name, description, created_at, updated_at
	`
	var c Competition
	if err := tx.QueryRow(ctx, insertComp, slug, name, description).Scan(
		&c.ID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		if strings.Contains(err.Error(), "idx_competitions_slug") || strings.Contains(err.Error(), "competitions_slug_key") {
			return nil, ErrSlugAlreadyExists
		}
		return nil, fmt.Errorf("insert competition: %w", err)
	}

	insertMember := `
		INSERT INTO user_competitions (competition_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`
	if _, err := tx.Exec(ctx, insertMember, c.ID, ownerUserID); err != nil {
		if strings.Contains(err.Error(), "idx_user_competitions_unique_owner") {
			return nil, ErrUserAlreadyOwnsCompetition
		}
		return nil, fmt.Errorf("insert competition owner: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &c, nil
}

// GetCompetitionBySlug retrieves a competition by its vanity slug.
func (r *Repository) GetCompetitionBySlug(ctx context.Context, slug string) (*Competition, error) {
	query := `
		SELECT id, slug, name, description, created_at, updated_at
		FROM competitions
		WHERE LOWER(slug) = LOWER($1)
	`
	var c Competition
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&c.ID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCompetitionNotFound
		}
		return nil, fmt.Errorf("get competition by slug: %w", err)
	}
	return &c, nil
}

// GetCompetitionByID retrieves a competition by its UUID.
func (r *Repository) GetCompetitionByID(ctx context.Context, id string) (*Competition, error) {
	query := `
		SELECT id, slug, name, description, created_at, updated_at
		FROM competitions
		WHERE id = $1
	`
	var c Competition
	err := r.db.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCompetitionNotFound
		}
		return nil, fmt.Errorf("get competition by id: %w", err)
	}
	return &c, nil
}

// UpdateCompetition updates metadata for a competition.
func (r *Repository) UpdateCompetition(ctx context.Context, id string, name, description *string) (*Competition, error) {
	var sets []string
	var args []any
	argIndex := 1

	if name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, *name)
		argIndex++
	}
	if description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", argIndex))
		args = append(args, *description)
		argIndex++
	}
	if len(sets) == 0 {
		return r.GetCompetitionByID(ctx, id)
	}

	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	query := fmt.Sprintf("UPDATE competitions SET %s WHERE id = $%d RETURNING id, slug, name, description, created_at, updated_at", strings.Join(sets, ", "), argIndex)
	args = append(args, id)

	var c Competition
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&c.ID, &c.Slug, &c.Name, &c.Description, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCompetitionNotFound
		}
		return nil, fmt.Errorf("update competition: %w", err)
	}
	return &c, nil
}

// TransferOwnership transfers ownership of a competition from currentOwnerID to newOwnerID.
// The previous owner is demoted to 'admin', and the new owner is assigned 'owner'.
// Enforces that newOwnerID cannot own any other competition.
func (r *Repository) TransferOwnership(ctx context.Context, competitionID, currentOwnerID, newOwnerID string) error {
	if currentOwnerID == newOwnerID {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Verify currentOwnerID is indeed the owner
	var currentRole string
	err = tx.QueryRow(ctx, `SELECT role::text FROM user_competitions WHERE competition_id = $1 AND user_id = $2 FOR UPDATE`, competitionID, currentOwnerID).Scan(&currentRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return fmt.Errorf("verify owner: %w", err)
	}
	if currentRole != "owner" {
		return errors.New("only the current owner can transfer ownership")
	}

	// Verify newOwnerID does not own any other competition
	var otherCompID string
	err = tx.QueryRow(ctx, `SELECT competition_id FROM user_competitions WHERE user_id = $1 AND role = 'owner' AND competition_id != $2 LIMIT 1`, newOwnerID, competitionID).Scan(&otherCompID)
	if err == nil {
		return ErrTargetAlreadyOwnsCompetition
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check target user ownership: %w", err)
	}

	// Verify target user exists and is active
	var targetUserExists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)`, newOwnerID).Scan(&targetUserExists)
	if err != nil || !targetUserExists {
		return errors.New("target user does not exist")
	}

	// Demote current owner to admin
	demoteQuery := `
		UPDATE user_competitions
		SET role = 'admin', updated_at = CURRENT_TIMESTAMP
		WHERE competition_id = $1 AND user_id = $2
	`
	if _, err := tx.Exec(ctx, demoteQuery, competitionID, currentOwnerID); err != nil {
		return fmt.Errorf("demote current owner: %w", err)
	}

	// Promote new owner
	promoteQuery := `
		INSERT INTO user_competitions (competition_id, user_id, role)
		VALUES ($1, $2, 'owner')
		ON CONFLICT (competition_id, user_id)
		DO UPDATE SET role = 'owner', updated_at = CURRENT_TIMESTAMP
	`
	if _, err := tx.Exec(ctx, promoteQuery, competitionID, newOwnerID); err != nil {
		if strings.Contains(err.Error(), "idx_user_competitions_unique_owner") {
			return ErrTargetAlreadyOwnsCompetition
		}
		return fmt.Errorf("promote new owner: %w", err)
	}

	return tx.Commit(ctx)
}

// AssignOrUpdateMemberRole sets a member's role in a competition.
// Owner promotion is forbidden here (must use TransferOwnership).
func (r *Repository) AssignOrUpdateMemberRole(ctx context.Context, competitionID, targetUserID, role string) error {
	if role == "owner" {
		return errors.New("cannot assign owner role via role update; use transfer ownership")
	}

	// Verify target user exists
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)`, targetUserID).Scan(&exists)
	if err != nil || !exists {
		return errors.New("target user does not exist")
	}

	// Ensure we are not silently demoting an existing owner
	var currentRole string
	err = r.db.QueryRow(ctx, `SELECT role::text FROM user_competitions WHERE competition_id = $1 AND user_id = $2`, competitionID, targetUserID).Scan(&currentRole)
	if err == nil && currentRole == "owner" {
		return errors.New("cannot demote owner; transfer ownership first")
	}

	query := `
		INSERT INTO user_competitions (competition_id, user_id, role)
		VALUES ($1, $2, $3::competition_role)
		ON CONFLICT (competition_id, user_id)
		DO UPDATE SET role = EXCLUDED.role, updated_at = CURRENT_TIMESTAMP
	`
	_, err = r.db.Exec(ctx, query, competitionID, targetUserID, role)
	if err != nil {
		return fmt.Errorf("assign member role: %w", err)
	}
	return nil
}

// RemoveMember removes a user from a competition.
// Owners cannot be removed; they must transfer ownership first.
func (r *Repository) RemoveMember(ctx context.Context, competitionID, targetUserID string) error {
	var role string
	err := r.db.QueryRow(ctx, `SELECT role::text FROM user_competitions WHERE competition_id = $1 AND user_id = $2`, competitionID, targetUserID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return fmt.Errorf("find member: %w", err)
	}
	if role == "owner" {
		return ErrCannotRemoveOwner
	}

	_, err = r.db.Exec(ctx, `DELETE FROM user_competitions WHERE competition_id = $1 AND user_id = $2`, competitionID, targetUserID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	return nil
}

// GetMemberRole gets the active role of a user in a competition.
func (r *Repository) GetMemberRole(ctx context.Context, competitionID, userID string) (string, error) {
	var role string
	err := r.db.QueryRow(ctx, `SELECT role::text FROM user_competitions WHERE competition_id = $1 AND user_id = $2`, competitionID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrMemberNotFound
		}
		return "", fmt.Errorf("get member role: %w", err)
	}
	return role, nil
}

// ListMembers lists all members belonging to a competition with an optional role filter.
func (r *Repository) ListMembers(ctx context.Context, competitionID, roleFilter string) ([]MemberWithUser, error) {
	var query string
	var args []any
	if roleFilter != "" {
		query = `
			SELECT uc.id, uc.competition_id, uc.user_id, u.email, u.name, u.phone_number, uc.role::text, uc.created_at, uc.updated_at
			FROM user_competitions uc
			JOIN users u ON uc.user_id = u.id
			WHERE uc.competition_id = $1 AND uc.role = $2::competition_role AND u.deleted_at IS NULL
			ORDER BY uc.created_at ASC
		`
		args = []any{competitionID, roleFilter}
	} else {
		query = `
			SELECT uc.id, uc.competition_id, uc.user_id, u.email, u.name, u.phone_number, uc.role::text, uc.created_at, uc.updated_at
			FROM user_competitions uc
			JOIN users u ON uc.user_id = u.id
			WHERE uc.competition_id = $1 AND u.deleted_at IS NULL
			ORDER BY uc.created_at ASC
		`
		args = []any{competitionID}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	members := make([]MemberWithUser, 0)
	for rows.Next() {
		var m MemberWithUser
		if err := rows.Scan(
			&m.ID, &m.CompetitionID, &m.UserID, &m.Email, &m.Name, &m.PhoneNumber, &m.Role, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate members: %w", err)
	}
	return members, nil
}
