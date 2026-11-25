package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrTokenNotFound      = errors.New("token not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// UserProvisioner encapsulates account provisioning and role updates owned by the users module.
type UserProvisioner interface {
	UpsertGoogleUser(ctx context.Context, googleID, email, name string, avatarURL *string, defaultRole string) (*User, error)
	SetRole(ctx context.Context, userID, role string) error
}

type Repository struct {
	db          *pgxpool.Pool
	provisioner UserProvisioner
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// WithUserProvisioner configures the users domain seam for OAuth user provisioning and role updates.
func (r *Repository) WithUserProvisioner(p UserProvisioner) *Repository {
	if p != nil {
		r.provisioner = p
	}
	return r
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, name, google_id, avatar_url, role, phone_number,
		       is_active, token_version, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`
	var u User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.Name, &u.GoogleID, &u.AvatarURL, &u.Role, &u.PhoneNumber,
		&u.IsActive, &u.TokenVersion, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `
		SELECT id, email, name, google_id, avatar_url, role, phone_number,
		       is_active, token_version, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.Name, &u.GoogleID, &u.AvatarURL, &u.Role, &u.PhoneNumber,
		&u.IsActive, &u.TokenVersion, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetUserByGoogleID(ctx context.Context, googleID string) (*User, error) {
	query := `
		SELECT id, email, name, google_id, avatar_url, role, phone_number,
		       is_active, token_version, created_at, updated_at
		FROM users
		WHERE google_id = $1
	`
	var u User
	err := r.db.QueryRow(ctx, query, googleID).Scan(
		&u.ID, &u.Email, &u.Name, &u.GoogleID, &u.AvatarURL, &u.Role, &u.PhoneNumber,
		&u.IsActive, &u.TokenVersion, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by google id: %w", err)
	}
	return &u, nil
}

// UpsertGoogleUser delegates Google OAuth account provisioning to users.Provisioner.
// Never overwrites is_active = FALSE (a blacklisted or soft-deleted user remains inactive until restored by an admin/dev).
func (r *Repository) UpsertGoogleUser(ctx context.Context, googleID, email, name string, avatarURL *string, defaultRole string) (*User, error) {
	if r.provisioner == nil {
		return nil, errors.New("user provisioner not configured")
	}
	return r.provisioner.UpsertGoogleUser(ctx, googleID, email, name, avatarURL, defaultRole)
}

// GetUserEvents retrieves all non-banned events and associated roles for a user.
// Platform devs automatically see all events with 'owner' role unless explicitly assigned another non-banned role.
func (r *Repository) GetUserEvents(ctx context.Context, userID string) ([]UserEventRole, error) {
	query := `
		SELECT e.id, e.slug, e.name, e.logo_url,
		       (SELECT COUNT(*) FROM user_events ue_c JOIN users u_c ON u_c.id = ue_c.user_id WHERE ue_c.event_id = e.id AND ue_c.is_banned = FALSE AND u_c.deleted_at IS NULL) AS member_count,
		       COALESCE(ue.role::text, 'owner') AS role
		FROM events e
		JOIN users u ON u.id = $1
		LEFT JOIN user_events ue ON ue.event_id = e.id AND ue.user_id = u.id
		WHERE (u.role = 'dev' OR ue.user_id IS NOT NULL)
		  AND COALESCE(ue.is_banned, FALSE) = FALSE
		ORDER BY e.created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user events: %w", err)
	}
	defer rows.Close()

	evs := make([]UserEventRole, 0)
	for rows.Next() {
		var item UserEventRole
		if err := rows.Scan(&item.EventID, &item.EventSlug, &item.EventName, &item.LogoURL, &item.MemberCount, &item.Role); err != nil {
			return nil, fmt.Errorf("scan user event: %w", err)
		}
		evs = append(evs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user events: %w", err)
	}
	return evs, nil
}

func (r *Repository) SetRole(ctx context.Context, userID, role string) error {
	if r.provisioner == nil {
		return errors.New("user provisioner not configured")
	}
	return r.provisioner.SetRole(ctx, userID, role)
}

func (r *Repository) CreateRefreshToken(ctx context.Context, s *RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (user_id, device_id, token_hash, family_id, parent_id, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, COALESCE(NULLIF($4, '')::uuid, gen_random_uuid()), $5, $6, $7, $8)
		RETURNING id, family_id, created_at
	`
	err := r.db.QueryRow(ctx, query,
		s.UserID, s.DeviceID, s.TokenHash, s.FamilyID, s.ParentID, s.UserAgent, s.IPAddress, s.ExpiresAt,
	).Scan(&s.ID, &s.FamilyID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

func (r *Repository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	query := `
		SELECT id, user_id, device_id, token_hash, family_id, parent_id, user_agent, ip_address,
		       is_revoked, expires_at, created_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var s RefreshToken
	err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&s.ID, &s.UserID, &s.DeviceID, &s.TokenHash, &s.FamilyID, &s.ParentID, &s.UserAgent, &s.IPAddress,
		&s.IsRevoked, &s.ExpiresAt, &s.CreatedAt, &s.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("get refresh token by hash: %w", err)
	}
	return &s, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	query := `
		UPDATE refresh_tokens
		SET is_revoked = TRUE, revoked_at = CURRENT_TIMESTAMP
		WHERE token_hash = $1
	`
	_, err := r.db.Exec(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (r *Repository) RevokeDeviceSessions(ctx context.Context, userID, deviceID string) error {
	query := `
		UPDATE refresh_tokens
		SET is_revoked = TRUE, revoked_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND device_id = $2 AND is_revoked = FALSE
	`
	_, err := r.db.Exec(ctx, query, userID, deviceID)
	if err != nil {
		return fmt.Errorf("revoke device sessions: %w", err)
	}
	return nil
}

func (r *Repository) RevokeAllUserTokens(ctx context.Context, userID string) error {
	query := `
		UPDATE refresh_tokens
		SET is_revoked = TRUE, revoked_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND is_revoked = FALSE
	`
	_, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("revoke all user tokens: %w", err)
	}
	return nil
}
