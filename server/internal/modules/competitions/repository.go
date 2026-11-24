package competitions

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

// deriveNameFromEmail converts an email address into a human-friendly display name.
func deriveNameFromEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) == 0 || parts[0] == "" {
		return "Competition Owner"
	}
	namePart := parts[0]
	namePart = strings.ReplaceAll(namePart, ".", " ")
	namePart = strings.ReplaceAll(namePart, "_", " ")
	namePart = strings.ReplaceAll(namePart, "-", " ")
	words := strings.Fields(namePart)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	if len(words) == 0 {
		return "Competition Owner"
	}
	return strings.Join(words, " ")
}

// GetOrCreateUserByEmail retrieves an existing user's ID by email, or creates a new user account in the background if they don't exist.
func (r *Repository) GetOrCreateUserByEmail(ctx context.Context, email, name string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", errors.New("owner email cannot be empty")
	}

	if strings.TrimSpace(name) == "" {
		name = deriveNameFromEmail(email)
	}

	query := `
		INSERT INTO users (email, name, global_role, is_active)
		VALUES ($1, $2, 'user', TRUE)
		ON CONFLICT (email) DO UPDATE
		SET deleted_at = NULL,
		    is_active = TRUE,
		    updated_at = CURRENT_TIMESTAMP
		RETURNING id;
	`
	var id string
	err := r.db.QueryRow(ctx, query, email, name).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("get or create owner account: %w", err)
	}
	return id, nil
}

// AddCompetitionUser inserts or links a user to a competition by email.
// If the user does not exist in users table, it creates their account in the background (returning created=true).
// If the user already exists, it links or updates their membership in this competition (returning created=false).
func (r *Repository) AddCompetitionUser(
	ctx context.Context,
	compID, actorID, actorRole string,
	email string,
	name *string,
	phone *string,
	role string,
) (*CompetitionUserDetailResponse, bool, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, false, errors.New("email cannot be empty")
	}
	if role == "" {
		role = "participant"
	}
	if role == "owner" {
		return nil, false, errors.New("cannot assign owner role; use transfer ownership endpoint")
	}

	// Role hierarchy check: admin cannot assign admin or owner
	if actorRole == "admin" && (role == "admin" || role == "owner") {
		return nil, false, ErrPermissionDenied
	}

	userName := ""
	if name != nil && strings.TrimSpace(*name) != "" {
		userName = strings.TrimSpace(*name)
	} else {
		userName = deriveNameFromEmail(email)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Step 1: Upsert user account in users table (detecting whether it was freshly created)
	upsertUserQuery := `
		INSERT INTO users (email, name, phone_number, global_role, is_active)
		VALUES ($1, $2, $3, 'user', TRUE)
		ON CONFLICT (email) DO UPDATE
		SET deleted_at = NULL,
		    is_active = CASE WHEN users.is_active = FALSE THEN TRUE ELSE users.is_active END,
		    phone_number = COALESCE(users.phone_number, EXCLUDED.phone_number),
		    updated_at = CURRENT_TIMESTAMP
		RETURNING id, (xmax = 0) AS created
	`
	var targetUserID string
	var createdAccount bool
	if err := tx.QueryRow(ctx, upsertUserQuery, email, userName, phone).Scan(&targetUserID, &createdAccount); err != nil {
		return nil, false, fmt.Errorf("upsert user: %w", err)
	}

	// Step 2: Check current membership status in this competition
	var existingRole string
	var isBanned bool
	checkQuery := `
		SELECT role::text, is_banned
		FROM user_competitions
		WHERE competition_id = $1 AND user_id = $2
		FOR UPDATE
	`
	err = tx.QueryRow(ctx, checkQuery, compID, targetUserID).Scan(&existingRole, &isBanned)
	if err == nil {
		// User is already in this competition
		if isBanned {
			return nil, false, errors.New("user is banned from this competition; unban first")
		}
		if existingRole == "owner" && role != "owner" {
			return nil, false, ErrCannotModifyOwner
		}
		if actorRole == "admin" && (existingRole == "admin" || existingRole == "owner") {
			return nil, false, ErrPermissionDenied
		}

		// Update role
		updateQuery := `
			UPDATE user_competitions
			SET role = $1::competition_role, updated_at = CURRENT_TIMESTAMP
			WHERE competition_id = $2 AND user_id = $3
		`
		if _, err := tx.Exec(ctx, updateQuery, role, compID, targetUserID); err != nil {
			return nil, false, fmt.Errorf("update member role: %w", err)
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		// User is not in this competition, insert membership
		insertQuery := `
			INSERT INTO user_competitions (competition_id, user_id, role)
			VALUES ($1, $2, $3::competition_role)
		`
		if _, err := tx.Exec(ctx, insertQuery, compID, targetUserID, role); err != nil {
			return nil, false, fmt.Errorf("insert member: %w", err)
		}
	} else {
		return nil, false, fmt.Errorf("check membership: %w", err)
	}

	// Step 3: Fetch detail response
	fetchQuery := `
		SELECT u.id, u.email, u.name, u.phone_number, u.global_role, u.avatar_url,
		       uc.role::text, uc.is_banned, uc.banned_at,
		       u.team_id, t.name AS team_name, (t.leader_id = u.id) AS is_leader,
		       u.created_at
		FROM users u
		JOIN user_competitions uc ON uc.user_id = u.id AND uc.competition_id = $1
		LEFT JOIN teams t ON u.team_id = t.id AND t.competition_id = $1
		WHERE u.id = $2
	`
	var detail CompetitionUserDetailResponse
	var isLeader *bool
	var bannedAt *time.Time
	err = tx.QueryRow(ctx, fetchQuery, compID, targetUserID).Scan(
		&detail.ID, &detail.Email, &detail.Name, &detail.PhoneNumber, &detail.GlobalRole, &detail.AvatarURL,
		&detail.Role, &detail.IsBanned, &bannedAt,
		&detail.TeamID, &detail.TeamName, &isLeader,
		&detail.CreatedAt,
	)
	if err != nil {
		return nil, false, fmt.Errorf("fetch created user detail: %w", err)
	}
	detail.BannedAt = bannedAt
	detail.IsLeader = isLeader

	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit tx: %w", err)
	}

	return &detail, createdAccount, nil
}

// GetUserIDByEmail retrieves the ID of an active user by email.
func (r *Repository) GetUserIDByEmail(ctx context.Context, email string) (string, error) {
	var id string
	query := `SELECT id FROM users WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL`
	err := r.db.QueryRow(ctx, query, email).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("owner user not found with specified email")
		}
		return "", fmt.Errorf("lookup user by email: %w", err)
	}
	return id, nil
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

// GetMemberRole gets the active role of a user in a competition along with ban status.
func (r *Repository) GetMemberRole(ctx context.Context, competitionID, userID string) (string, bool, error) {
	var role string
	var isBanned bool
	err := r.db.QueryRow(ctx, `SELECT role::text, is_banned FROM user_competitions WHERE competition_id = $1 AND user_id = $2`, competitionID, userID).Scan(&role, &isBanned)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, ErrMemberNotFound
		}
		return "", false, fmt.Errorf("get member role: %w", err)
	}
	return role, isBanned, nil
}

// BanMember sets is_banned=TRUE on the user's membership. Competition owners cannot be banned.
func (r *Repository) BanMember(ctx context.Context, competitionID, targetUserID string) error {
	var role string
	err := r.db.QueryRow(ctx, `SELECT role::text FROM user_competitions WHERE competition_id = $1 AND user_id = $2`, competitionID, targetUserID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return fmt.Errorf("find member to ban: %w", err)
	}
	if role == "owner" {
		return errors.New("cannot ban the competition owner")
	}

	_, err = r.db.Exec(ctx, `
		UPDATE user_competitions
		SET is_banned = TRUE, banned_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE competition_id = $1 AND user_id = $2
	`, competitionID, targetUserID)
	if err != nil {
		return fmt.Errorf("ban member: %w", err)
	}
	return nil
}

// UnbanMember restores a banned member's active status.
func (r *Repository) UnbanMember(ctx context.Context, competitionID, targetUserID string) (string, error) {
	var role string
	var isBanned bool
	err := r.db.QueryRow(ctx, `SELECT role::text, is_banned FROM user_competitions WHERE competition_id = $1 AND user_id = $2`, competitionID, targetUserID).Scan(&role, &isBanned)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrMemberNotFound
		}
		return "", fmt.Errorf("find member to unban: %w", err)
	}
	if !isBanned {
		return role, nil
	}

	_, err = r.db.Exec(ctx, `
		UPDATE user_competitions
		SET is_banned = FALSE, banned_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE competition_id = $1 AND user_id = $2
	`, competitionID, targetUserID)
	if err != nil {
		return "", fmt.Errorf("unban member: %w", err)
	}
	return role, nil
}

// ListMembers lists all members belonging to a competition with an optional role filter.
func (r *Repository) ListMembers(ctx context.Context, competitionID, roleFilter string) ([]MemberWithUser, error) {
	var query string
	var args []any
	if roleFilter != "" {
		query = `
			SELECT uc.id, uc.competition_id, uc.user_id, u.email, u.name, u.phone_number, uc.role::text, uc.is_banned, uc.banned_at, uc.created_at, uc.updated_at
			FROM user_competitions uc
			JOIN users u ON uc.user_id = u.id
			WHERE uc.competition_id = $1 AND uc.role = $2::competition_role AND u.deleted_at IS NULL
			ORDER BY uc.created_at ASC
		`
		args = []any{competitionID, roleFilter}
	} else {
		query = `
			SELECT uc.id, uc.competition_id, uc.user_id, u.email, u.name, u.phone_number, uc.role::text, uc.is_banned, uc.banned_at, uc.created_at, uc.updated_at
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
			&m.ID, &m.CompetitionID, &m.UserID, &m.Email, &m.Name, &m.PhoneNumber, &m.Role, &m.IsBanned, &m.BannedAt, &m.CreatedAt, &m.UpdatedAt,
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

// ListCompetitionUsers returns paginated user details scoped exclusively to this competition.
func (r *Repository) ListCompetitionUsers(ctx context.Context, competitionID string, filter CompetitionUserFilter) ([]CompetitionUserDetailResponse, int64, error) {
	conditions := []string{"uc.competition_id = $1", "u.deleted_at IS NULL"}
	args := []any{competitionID}
	argIndex := 2

	if filter.Role != "" {
		conditions = append(conditions, fmt.Sprintf("uc.role = $%d::competition_role", argIndex))
		args = append(args, filter.Role)
		argIndex++
	}

	if filter.IsBanned != nil {
		conditions = append(conditions, fmt.Sprintf("uc.is_banned = $%d", argIndex))
		args = append(args, *filter.IsBanned)
		argIndex++
	}

	if filter.TeamID != "" {
		conditions = append(conditions, fmt.Sprintf("u.team_id = $%d", argIndex))
		args = append(args, filter.TeamID)
		argIndex++
	}

	if filter.NoTeam {
		conditions = append(conditions, "u.team_id IS NULL")
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

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM user_competitions uc
		JOIN users u ON uc.user_id = u.id
		LEFT JOIN tickets tk ON tk.user_id = u.id AND tk.competition_id = uc.competition_id
		WHERE %s
	`, whereClause)

	var total int64
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count competition users: %w", err)
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
		SELECT u.id, u.email, u.name, u.phone_number, u.global_role, u.avatar_url,
		       uc.role::text, uc.is_banned, uc.banned_at,
		       u.team_id, t.name, (t.leader_id = u.id) AS is_leader,
		       u.created_at,
		       CASE WHEN tk.id IS NULL THEN 'no_pass' WHEN tk.is_checked_in THEN 'checked_in' ELSE 'not_checked_in' END,
		       tk.checked_in_at
		FROM user_competitions uc
		JOIN users u ON uc.user_id = u.id
		LEFT JOIN teams t ON u.team_id = t.id AND t.competition_id = uc.competition_id
		LEFT JOIN tickets tk ON tk.user_id = u.id AND tk.competition_id = uc.competition_id
		WHERE %s
		ORDER BY uc.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list competition users: %w", err)
	}
	defer rows.Close()

	usersList := make([]CompetitionUserDetailResponse, 0, limit)
	for rows.Next() {
		var u CompetitionUserDetailResponse
		var teamName *string
		var isLeader *bool
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Name, &u.PhoneNumber, &u.GlobalRole, &u.AvatarURL,
			&u.Role, &u.IsBanned, &u.BannedAt,
			&u.TeamID, &teamName, &isLeader,
			&u.CreatedAt,
			&u.CheckinStatus, &u.CheckedInAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan competition user: %w", err)
		}
		u.TeamName = teamName
		u.IsLeader = isLeader
		usersList = append(usersList, u)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	return usersList, total, nil
}
