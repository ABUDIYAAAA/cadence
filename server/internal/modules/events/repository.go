package events

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"devclubxnst.com/cadence/pkg/audit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEventNotFound          = errors.New("event not found")
	ErrSlugAlreadyExists      = errors.New("event slug already in use")
	ErrUserAlreadyOwnsEvent   = errors.New("user already owns an event")
	ErrTargetAlreadyOwnsEvent = errors.New("target user already owns another event")
	ErrCannotRemoveOwner      = errors.New("cannot remove event owner; transfer ownership first")
	ErrMemberNotFound         = errors.New("event member not found")
)

// TeamEvictor encapsulates team eviction within an event transaction and records any resulting team audit events after commit.
type TeamEvictor interface {
	EvictAndAuditEventTeamsTx(ctx context.Context, tx pgx.Tx, eventID, userID string, auditor audit.Recorder) (func(), error)
}

type Repository struct {
	db      *pgxpool.Pool
	teams   TeamEvictor
	auditor audit.Recorder
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

// WithTeamEvictor configures the teams domain seam used for transactional member eviction.
func (r *Repository) WithTeamEvictor(evictor TeamEvictor) *Repository {
	if evictor != nil {
		r.teams = evictor
	}
	return r
}

// WithAuditor attaches an audit recorder so team dissolutions and leadership transfers during event evictions are audited.
func (r *Repository) WithAuditor(auditor audit.Recorder) *Repository {
	r.auditor = auditor
	return r
}

// deriveNameFromEmail converts an email address into a human-friendly display name.
func deriveNameFromEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) == 0 || parts[0] == "" {
		return "Event Owner"
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
		return "Event Owner"
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
		INSERT INTO users (email, name, role, is_active)
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

// CreateEvent inserts a new event and binds the designated user as 'owner' within a single transaction.
// A user is strictly prevented from owning more than one event simultaneously.
func (r *Repository) CreateEvent(ctx context.Context, slug, name string, description, logoURL *string, ownerUserID string) (*Event, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check if user is already an owner of any event
	var existingEventID string
	checkOwnerQuery := `SELECT event_id FROM user_events WHERE user_id = $1 AND role = 'owner' LIMIT 1`
	err = tx.QueryRow(ctx, checkOwnerQuery, ownerUserID).Scan(&existingEventID)
	if err == nil {
		return nil, ErrUserAlreadyOwnsEvent
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("check existing ownership: %w", err)
	}

	insertEv := `
		INSERT INTO events (slug, name, description, logo_url)
		VALUES (LOWER($1), $2, $3, $4)
		RETURNING id, slug, name, description, logo_url, created_at, updated_at
	`
	var ev Event
	if err := tx.QueryRow(ctx, insertEv, slug, name, description, logoURL).Scan(
		&ev.ID, &ev.Slug, &ev.Name, &ev.Description, &ev.LogoURL, &ev.CreatedAt, &ev.UpdatedAt,
	); err != nil {
		if strings.Contains(err.Error(), "idx_events_slug") || strings.Contains(err.Error(), "events_slug_key") {
			return nil, ErrSlugAlreadyExists
		}
		return nil, fmt.Errorf("insert event: %w", err)
	}

	insertMember := `
		INSERT INTO user_events (event_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`
	if _, err := tx.Exec(ctx, insertMember, ev.ID, ownerUserID); err != nil {
		if strings.Contains(err.Error(), "idx_user_events_unique_owner") {
			return nil, ErrUserAlreadyOwnsEvent
		}
		return nil, fmt.Errorf("insert event owner: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &ev, nil
}

// DeleteEvent deletes an event by ID (restricted to devs). Cascades to competitions, memberships, teams, etc.
func (r *Repository) DeleteEvent(ctx context.Context, eventID string) (*Event, error) {
	ev, err := r.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM events WHERE id = $1`, eventID)
	if err != nil {
		return nil, fmt.Errorf("delete event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrEventNotFound
	}
	return ev, nil
}

// ListEvents returns all events ordered by creation time.
func (r *Repository) ListEvents(ctx context.Context) ([]Event, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.id, e.slug, e.name, e.description, e.logo_url,
		       (SELECT COUNT(*) FROM user_events ue JOIN users u ON u.id = ue.user_id WHERE ue.event_id = e.id AND ue.is_banned = FALSE AND u.deleted_at IS NULL) AS member_count,
		       e.created_at, e.updated_at
		FROM events e
		ORDER BY e.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	list := make([]Event, 0)
	for rows.Next() {
		var ev Event
		if err := rows.Scan(&ev.ID, &ev.Slug, &ev.Name, &ev.Description, &ev.LogoURL, &ev.MemberCount, &ev.CreatedAt, &ev.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		list = append(list, ev)
	}
	return list, rows.Err()
}

// GetEventBySlug retrieves an event by its vanity slug.
func (r *Repository) GetEventBySlug(ctx context.Context, slug string) (*Event, error) {
	query := `
		SELECT id, slug, name, description, logo_url, created_at, updated_at
		FROM events
		WHERE LOWER(slug) = LOWER($1)
	`
	var ev Event
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&ev.ID, &ev.Slug, &ev.Name, &ev.Description, &ev.LogoURL, &ev.CreatedAt, &ev.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEventNotFound
		}
		return nil, fmt.Errorf("get event by slug: %w", err)
	}
	return &ev, nil
}

// GetEventByID retrieves an event by its UUID.
func (r *Repository) GetEventByID(ctx context.Context, id string) (*Event, error) {
	query := `
		SELECT id, slug, name, description, logo_url, created_at, updated_at
		FROM events
		WHERE id = $1
	`
	var ev Event
	err := r.db.QueryRow(ctx, query, id).Scan(
		&ev.ID, &ev.Slug, &ev.Name, &ev.Description, &ev.LogoURL, &ev.CreatedAt, &ev.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEventNotFound
		}
		return nil, fmt.Errorf("get event by id: %w", err)
	}
	return &ev, nil
}

// UpdateEvent updates metadata and logo/photo for an event.
func (r *Repository) UpdateEvent(ctx context.Context, id string, name, description, logoURL *string) (*Event, error) {
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
	if logoURL != nil {
		trimmed := strings.TrimSpace(*logoURL)
		if trimmed == "" {
			sets = append(sets, "logo_url = NULL")
		} else {
			sets = append(sets, fmt.Sprintf("logo_url = $%d", argIndex))
			args = append(args, trimmed)
			argIndex++
		}
	}
	if len(sets) == 0 {
		return r.GetEventByID(ctx, id)
	}

	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	query := fmt.Sprintf("UPDATE events SET %s WHERE id = $%d RETURNING id, slug, name, description, logo_url, created_at, updated_at", strings.Join(sets, ", "), argIndex)
	args = append(args, id)

	var ev Event
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&ev.ID, &ev.Slug, &ev.Name, &ev.Description, &ev.LogoURL, &ev.CreatedAt, &ev.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEventNotFound
		}
		return nil, fmt.Errorf("update event: %w", err)
	}
	return &ev, nil
}

// TransferOwnership transfers ownership of an event from currentOwnerID to newOwnerID.
// The previous owner is demoted to 'admin', and the new owner is assigned 'owner'.
// Enforces that newOwnerID cannot own any other event.
func (r *Repository) TransferOwnership(ctx context.Context, eventID, currentOwnerID, newOwnerID string) error {
	if currentOwnerID == newOwnerID {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var currentRole string
	err = tx.QueryRow(ctx, `SELECT role::text FROM user_events WHERE event_id = $1 AND user_id = $2 FOR UPDATE`, eventID, currentOwnerID).Scan(&currentRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return fmt.Errorf("verify owner: %w", err)
	}
	if currentRole != "owner" {
		return errors.New("only the current owner can transfer ownership")
	}

	// Verify newOwnerID does not own any other event
	var otherEventID string
	err = tx.QueryRow(ctx, `SELECT event_id FROM user_events WHERE user_id = $1 AND role = 'owner' AND event_id != $2 LIMIT 1`, newOwnerID, eventID).Scan(&otherEventID)
	if err == nil {
		return ErrTargetAlreadyOwnsEvent
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
		UPDATE user_events
		SET role = 'admin', updated_at = CURRENT_TIMESTAMP
		WHERE event_id = $1 AND user_id = $2
	`
	if _, err := tx.Exec(ctx, demoteQuery, eventID, currentOwnerID); err != nil {
		return fmt.Errorf("demote current owner: %w", err)
	}

	// Promote new owner
	promoteQuery := `
		INSERT INTO user_events (event_id, user_id, role)
		VALUES ($1, $2, 'owner')
		ON CONFLICT (event_id, user_id)
		DO UPDATE SET role = 'owner', updated_at = CURRENT_TIMESTAMP
	`
	if _, err := tx.Exec(ctx, promoteQuery, eventID, newOwnerID); err != nil {
		if strings.Contains(err.Error(), "idx_user_events_unique_owner") {
			return ErrTargetAlreadyOwnsEvent
		}
		return fmt.Errorf("promote new owner: %w", err)
	}

	return tx.Commit(ctx)
}

// AddEventUser inserts or links a user to an event by email.
// If the user does not exist in users table, it creates their account in the background (returning created=true).
// If the user already exists, it links or updates their membership in this event (returning created=false).
func (r *Repository) AddEventUser(
	ctx context.Context,
	eventID, actorID, actorRole string,
	email string,
	name *string,
	phone *string,
	role string,
) (*EventUserDetailResponse, bool, error) {
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

	// Step 1: Upsert user account in users table
	upsertUserQuery := `
		INSERT INTO users (email, name, phone_number, role, is_active)
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

	// Step 2: Check current membership status in this event
	var existingRole string
	var isBanned bool
	checkQuery := `
		SELECT role::text, is_banned
		FROM user_events
		WHERE event_id = $1 AND user_id = $2
		FOR UPDATE
	`
	err = tx.QueryRow(ctx, checkQuery, eventID, targetUserID).Scan(&existingRole, &isBanned)
	if err == nil {
		if isBanned {
			return nil, false, errors.New("user is banned from this event; unban first")
		}
		if existingRole == "owner" && role != "owner" {
			return nil, false, ErrCannotModifyOwner
		}
		if actorRole == "admin" && (existingRole == "admin" || existingRole == "owner") {
			return nil, false, ErrPermissionDenied
		}

		updateQuery := `
			UPDATE user_events
			SET role = $1::event_role, updated_at = CURRENT_TIMESTAMP
			WHERE event_id = $2 AND user_id = $3
		`
		if _, err := tx.Exec(ctx, updateQuery, role, eventID, targetUserID); err != nil {
			return nil, false, fmt.Errorf("update member role: %w", err)
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		insertQuery := `
			INSERT INTO user_events (event_id, user_id, role)
			VALUES ($1, $2, $3::event_role)
		`
		if _, err := tx.Exec(ctx, insertQuery, eventID, targetUserID, role); err != nil {
			return nil, false, fmt.Errorf("insert member: %w", err)
		}
	} else {
		return nil, false, fmt.Errorf("check membership: %w", err)
	}

	// Step 3: Fetch detail response
	fetchQuery := `
		SELECT u.id, u.email, u.name, u.phone_number, u.avatar_url,
		       ue.role::text, ue.is_banned, ue.banned_at,
		       tm_sub.team_id, tm_sub.team_name, tm_sub.is_leader,
		       u.created_at
		FROM users u
		JOIN user_events ue ON ue.user_id = u.id AND ue.event_id = $1
		LEFT JOIN LATERAL (
			SELECT t.id AS team_id, t.name AS team_name, (t.leader_id = u.id) AS is_leader
			FROM team_members tm
			JOIN teams t ON t.id = tm.team_id
			JOIN competitions c ON c.id = t.competition_id
			WHERE tm.user_id = u.id AND c.event_id = $1
			ORDER BY tm.joined_at DESC
			LIMIT 1
		) tm_sub ON TRUE
		WHERE u.id = $2
	`
	var detail EventUserDetailResponse
	var isLeader *bool
	var bannedAt *time.Time
	err = tx.QueryRow(ctx, fetchQuery, eventID, targetUserID).Scan(
		&detail.ID, &detail.Email, &detail.Name, &detail.PhoneNumber, &detail.AvatarURL,
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

// AssignOrUpdateMemberRole sets a member's role in an event.
func (r *Repository) AssignOrUpdateMemberRole(ctx context.Context, eventID, targetUserID, role string) error {
	if role == "owner" {
		return errors.New("cannot assign owner role via role update; use transfer ownership")
	}

	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)`, targetUserID).Scan(&exists)
	if err != nil || !exists {
		return errors.New("target user does not exist")
	}

	var currentRole string
	err = r.db.QueryRow(ctx, `SELECT role::text FROM user_events WHERE event_id = $1 AND user_id = $2`, eventID, targetUserID).Scan(&currentRole)
	if err == nil && currentRole == "owner" {
		return errors.New("cannot demote owner; transfer ownership first")
	}

	query := `
		INSERT INTO user_events (event_id, user_id, role)
		VALUES ($1, $2, $3::event_role)
		ON CONFLICT (event_id, user_id)
		DO UPDATE SET role = EXCLUDED.role, updated_at = CURRENT_TIMESTAMP
	`
	_, err = r.db.Exec(ctx, query, eventID, targetUserID, role)
	if err != nil {
		return fmt.Errorf("assign member role: %w", err)
	}
	return nil
}

// RemoveMember removes a user from an event and evicts them from all teams in competitions belonging to this event.
func (r *Repository) RemoveMember(ctx context.Context, eventID, targetUserID string) error {
	var role string
	err := r.db.QueryRow(ctx, `SELECT role::text FROM user_events WHERE event_id = $1 AND user_id = $2`, eventID, targetUserID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return fmt.Errorf("find member: %w", err)
	}
	if role == "owner" {
		return ErrCannotRemoveOwner
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin remove member tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var postCommitAudit func()
	if r.teams != nil {
		postCommitAudit, err = r.teams.EvictAndAuditEventTeamsTx(ctx, tx, eventID, targetUserID, r.auditor)
		if err != nil {
			return fmt.Errorf("evict removed member from event teams: %w", err)
		}
	}

	_, err = tx.Exec(ctx, `DELETE FROM user_events WHERE event_id = $1 AND user_id = $2`, eventID, targetUserID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if postCommitAudit != nil {
		postCommitAudit()
	}
	return nil
}

// GetMemberRole gets the active role of a user in an event along with ban status.
func (r *Repository) GetMemberRole(ctx context.Context, eventID, userID string) (string, bool, error) {
	var role string
	var isBanned bool
	err := r.db.QueryRow(ctx, `SELECT role::text, is_banned FROM user_events WHERE event_id = $1 AND user_id = $2`, eventID, userID).Scan(&role, &isBanned)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, ErrMemberNotFound
		}
		return "", false, fmt.Errorf("get member role: %w", err)
	}
	return role, isBanned, nil
}

// BanMember sets is_banned=TRUE on the user's event membership and evicts them from all teams across every competition in this event.
func (r *Repository) BanMember(ctx context.Context, eventID, targetUserID string) error {
	var role string
	err := r.db.QueryRow(ctx, `SELECT role::text FROM user_events WHERE event_id = $1 AND user_id = $2`, eventID, targetUserID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return fmt.Errorf("find member to ban: %w", err)
	}
	if role == "owner" {
		return errors.New("cannot ban the event owner")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ban member tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE user_events
		SET is_banned = TRUE, banned_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE event_id = $1 AND user_id = $2
	`, eventID, targetUserID)
	if err != nil {
		return fmt.Errorf("ban member: %w", err)
	}

	var postCommitAudit func()
	if r.teams != nil {
		postCommitAudit, err = r.teams.EvictAndAuditEventTeamsTx(ctx, tx, eventID, targetUserID, r.auditor)
		if err != nil {
			return fmt.Errorf("evict banned member from event teams: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if postCommitAudit != nil {
		postCommitAudit()
	}
	return nil
}

// UnbanMember restores a banned member's active status in the event.
func (r *Repository) UnbanMember(ctx context.Context, eventID, targetUserID string) (string, error) {
	var role string
	var isBanned bool
	err := r.db.QueryRow(ctx, `SELECT role::text, is_banned FROM user_events WHERE event_id = $1 AND user_id = $2`, eventID, targetUserID).Scan(&role, &isBanned)
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
		UPDATE user_events
		SET is_banned = FALSE, banned_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE event_id = $1 AND user_id = $2
	`, eventID, targetUserID)
	if err != nil {
		return "", fmt.Errorf("unban member: %w", err)
	}
	return role, nil
}

// ListMembers lists all members belonging to an event with an optional role filter.
func (r *Repository) ListMembers(ctx context.Context, eventID, roleFilter string) ([]MemberWithUser, error) {
	var query string
	var args []any
	if roleFilter != "" {
		query = `
			SELECT ue.id, ue.event_id, ue.user_id, u.email, u.name, u.phone_number, ue.role::text, ue.is_banned, ue.banned_at, ue.created_at, ue.updated_at
			FROM user_events ue
			JOIN users u ON ue.user_id = u.id
			WHERE ue.event_id = $1 AND ue.role = $2::event_role AND u.deleted_at IS NULL
			ORDER BY ue.created_at ASC
		`
		args = []any{eventID, roleFilter}
	} else {
		query = `
			SELECT ue.id, ue.event_id, ue.user_id, u.email, u.name, u.phone_number, ue.role::text, ue.is_banned, ue.banned_at, ue.created_at, ue.updated_at
			FROM user_events ue
			JOIN users u ON ue.user_id = u.id
			WHERE ue.event_id = $1 AND u.deleted_at IS NULL
			ORDER BY ue.created_at ASC
		`
		args = []any{eventID}
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
			&m.ID, &m.EventID, &m.UserID, &m.Email, &m.Name, &m.PhoneNumber, &m.Role, &m.IsBanned, &m.BannedAt, &m.CreatedAt, &m.UpdatedAt,
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

// ListEventUsers returns paginated user details scoped exclusively to this event.
func (r *Repository) ListEventUsers(ctx context.Context, eventID string, filter EventUserFilter) ([]EventUserDetailResponse, int64, error) {
	conditions := []string{"ue.event_id = $1", "u.deleted_at IS NULL"}
	args := []any{eventID}
	argIndex := 2

	if filter.Role != "" {
		conditions = append(conditions, fmt.Sprintf("ue.role = $%d::event_role", argIndex))
		args = append(args, filter.Role)
		argIndex++
	}

	if filter.IsBanned != nil {
		conditions = append(conditions, fmt.Sprintf("ue.is_banned = $%d", argIndex))
		args = append(args, *filter.IsBanned)
		argIndex++
	}

	if filter.TeamID != "" {
		conditions = append(conditions, fmt.Sprintf("EXISTS (SELECT 1 FROM team_members tm WHERE tm.user_id = u.id AND tm.team_id = $%d)", argIndex))
		args = append(args, filter.TeamID)
		argIndex++
	}

	if filter.NoTeam {
		if filter.CompetitionID != "" {
			conditions = append(conditions, fmt.Sprintf("NOT EXISTS (SELECT 1 FROM team_members tm WHERE tm.user_id = u.id AND tm.competition_id = $%d)", argIndex))
			args = append(args, filter.CompetitionID)
			argIndex++
		} else {
			conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM team_members tm JOIN competitions c ON c.id = tm.competition_id WHERE tm.user_id = u.id AND c.event_id = ue.event_id)")
		}
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
		FROM user_events ue
		JOIN users u ON ue.user_id = u.id
		LEFT JOIN tickets tk ON tk.user_id = u.id AND tk.event_id = ue.event_id
		WHERE %s
	`, whereClause)

	var total int64
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count event users: %w", err)
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
		SELECT u.id, u.email, u.name, u.phone_number, u.avatar_url,
		       ue.role::text, ue.is_banned, ue.banned_at,
		       tm_sub.team_id, tm_sub.team_name, tm_sub.is_leader,
		       u.created_at,
		       CASE WHEN tk.id IS NULL THEN 'no_pass' WHEN tk.is_checked_in THEN 'checked_in' ELSE 'not_checked_in' END,
		       tk.checked_in_at
		FROM user_events ue
		JOIN users u ON ue.user_id = u.id
		LEFT JOIN LATERAL (
			SELECT t.id AS team_id, t.name AS team_name, (t.leader_id = u.id) AS is_leader
			FROM team_members tm
			JOIN teams t ON t.id = tm.team_id
			JOIN competitions c ON c.id = t.competition_id
			WHERE tm.user_id = u.id
			  AND c.event_id = ue.event_id
			  AND ($%d = '' OR tm.competition_id = $%d::uuid)
			  AND ($%d = '' OR tm.team_id = $%d::uuid)
			ORDER BY tm.joined_at DESC
			LIMIT 1
		) tm_sub ON TRUE
		LEFT JOIN tickets tk ON tk.user_id = u.id AND tk.event_id = ue.event_id
		WHERE %s
		ORDER BY ue.created_at DESC
		LIMIT $%d OFFSET $%d
	`, argIndex, argIndex, argIndex+1, argIndex+1, whereClause, argIndex+2, argIndex+3)

	args = append(args, filter.CompetitionID, filter.TeamID, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list event users: %w", err)
	}
	defer rows.Close()

	usersList := make([]EventUserDetailResponse, 0, limit)
	for rows.Next() {
		var u EventUserDetailResponse
		var teamName *string
		var isLeader *bool
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Name, &u.PhoneNumber, &u.AvatarURL,
			&u.Role, &u.IsBanned, &u.BannedAt,
			&u.TeamID, &teamName, &isLeader,
			&u.CreatedAt,
			&u.CheckinStatus, &u.CheckedInAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan event user: %w", err)
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
