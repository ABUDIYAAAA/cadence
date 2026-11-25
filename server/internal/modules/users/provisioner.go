package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"devclubxnst.com/cadence/internal/modules/auth"
	"github.com/jackc/pgx/v5"
)

// ProvisionedUser is the canonical result of finding, creating, or restoring a user account.
type ProvisionedUser struct {
	ID           string
	Email        string
	Name         string
	GoogleID     *string
	AvatarURL    *string
	Role         string
	PhoneNumber  *string
	IsActive     bool
	TokenVersion int
	Created      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Provisioner is the cross-module seam owned by users for account provisioning,
// soft-delete restoration, OAuth identity linking, role updates, and Redis blocklist/token-version sync.
type Provisioner interface {
	ProvisionByEmail(ctx context.Context, email, name string, phone *string) (*ProvisionedUser, error)
	ProvisionByEmailTx(ctx context.Context, tx pgx.Tx, email, name string, phone *string) (*ProvisionedUser, error)
	ProvisionOAuthUser(ctx context.Context, googleID, email, name string, avatarURL *string, defaultRole string) (*ProvisionedUser, error)
	SetUserRole(ctx context.Context, userID, role string) error
	SyncProvisionedUserRedis(ctx context.Context, u *ProvisionedUser)
}

var _ Provisioner = (*Repository)(nil)

// DeriveNameFromEmail converts an email address into a human-friendly display name.
func DeriveNameFromEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) == 0 || parts[0] == "" {
		return "User"
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
		return "User"
	}
	return strings.Join(words, " ")
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func provisionByEmailQuery(ctx context.Context, q rowQuerier, email, name string, phone *string) (*ProvisionedUser, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, errors.New("email cannot be empty")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = DeriveNameFromEmail(email)
	}

	// Soft-deleted accounts (deleted_at IS NOT NULL) are restored to is_active = TRUE with
	// an incremented token_version; platform-blacklisted accounts (is_active = FALSE AND
	// deleted_at IS NULL) remain inactive and are never un-blacklisted by provisioning.
	const query = `
		INSERT INTO users (email, name, phone_number, role, is_active)
		VALUES ($1, $2, $3, 'user', TRUE)
		ON CONFLICT (email) DO UPDATE
		SET is_active = CASE WHEN users.deleted_at IS NOT NULL THEN TRUE ELSE users.is_active END,
		    token_version = CASE WHEN users.deleted_at IS NOT NULL THEN users.token_version + 1 ELSE users.token_version END,
		    deleted_at = NULL,
		    phone_number = COALESCE(users.phone_number, EXCLUDED.phone_number),
		    updated_at = CURRENT_TIMESTAMP
		RETURNING id, email, name, google_id, avatar_url, role, phone_number,
		          is_active, token_version, (xmax = 0) AS created, created_at, updated_at
	`
	var u ProvisionedUser
	if err := q.QueryRow(ctx, query, email, name, phone).Scan(
		&u.ID, &u.Email, &u.Name, &u.GoogleID, &u.AvatarURL, &u.Role, &u.PhoneNumber,
		&u.IsActive, &u.TokenVersion, &u.Created, &u.CreatedAt, &u.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("provision user by email: %w", err)
	}
	return &u, nil
}

// ProvisionByEmail upserts a user by email outside an external transaction and synchronizes Redis state.
func (r *Repository) ProvisionByEmail(ctx context.Context, email, name string, phone *string) (*ProvisionedUser, error) {
	u, err := provisionByEmailQuery(ctx, r.db, email, name, phone)
	if err != nil {
		return nil, err
	}
	r.SyncProvisionedUserRedis(ctx, u)
	return u, nil
}

// ProvisionByEmailTx upserts a user by email inside tx. Callers should invoke
// SyncProvisionedUserRedis after tx.Commit succeeds.
func (r *Repository) ProvisionByEmailTx(ctx context.Context, tx pgx.Tx, email, name string, phone *string) (*ProvisionedUser, error) {
	return provisionByEmailQuery(ctx, tx, email, name, phone)
}

// ProvisionOAuthUser creates or updates a user via Google OAuth identity.
// Never overwrites is_active = FALSE (a blacklisted or soft-deleted user remains inactive until restored).
func (r *Repository) ProvisionOAuthUser(ctx context.Context, googleID, email, name string, avatarURL *string, defaultRole string) (*ProvisionedUser, error) {
	if defaultRole == "" {
		defaultRole = "user"
	}

	const query = `
		INSERT INTO users (email, name, google_id, avatar_url, role, is_active)
		VALUES (LOWER($1), $2, $3, $4, $5, TRUE)
		ON CONFLICT (email) DO UPDATE
		SET name = CASE WHEN users.name = '' THEN EXCLUDED.name ELSE users.name END,
		    google_id = COALESCE(users.google_id, EXCLUDED.google_id),
		    avatar_url = COALESCE(EXCLUDED.avatar_url, users.avatar_url),
		    role = CASE WHEN users.role = 'dev' THEN 'dev' ELSE EXCLUDED.role END,
		    updated_at = CURRENT_TIMESTAMP
		RETURNING id, email, name, google_id, avatar_url, role, phone_number,
		          is_active, token_version, (xmax = 0) AS created, created_at, updated_at
	`
	var u ProvisionedUser
	if err := r.db.QueryRow(ctx, query, email, name, googleID, avatarURL, defaultRole).Scan(
		&u.ID, &u.Email, &u.Name, &u.GoogleID, &u.AvatarURL, &u.Role, &u.PhoneNumber,
		&u.IsActive, &u.TokenVersion, &u.Created, &u.CreatedAt, &u.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("provision oauth user: %w", err)
	}
	return &u, nil
}

// SetUserRole updates a user's platform role, increments token_version, and synchronizes min_token_version in Redis.
func (r *Repository) SetUserRole(ctx context.Context, userID, role string) error {
	const query = `
		UPDATE users
		SET role = $1, token_version = token_version + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
		RETURNING token_version
	`
	var newVersion int
	if err := r.db.QueryRow(ctx, query, role, userID).Scan(&newVersion); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("set user role: %w", err)
	}
	if r.rdb != nil {
		updateMinTokenVersion(ctx, r.rdb, userID, newVersion, r.tokenTTL)
	}
	return nil
}

// SyncProvisionedUserRedis clears a stale Redis "deleted" block key when a soft-deleted
// account is restored (preserving "blacklisted" and "deactivated" blocks) and updates
// user:min_token_version monotonically so pre-deletion JWTs stay revoked.
func (r *Repository) SyncProvisionedUserRedis(ctx context.Context, u *ProvisionedUser) {
	if r.rdb == nil || u == nil || u.ID == "" || !u.IsActive {
		return
	}
	blockedKey := fmt.Sprintf("blocked:user:%s", u.ID)
	if val, err := r.rdb.Get(ctx, blockedKey).Result(); err == nil && val == "deleted" {
		_ = r.rdb.Del(ctx, blockedKey).Err()
	}
	if u.TokenVersion > 1 {
		updateMinTokenVersion(ctx, r.rdb, u.ID, u.TokenVersion, r.tokenTTL)
	}
}

type authProvisionerAdapter struct {
	p Provisioner
}

// NewAuthProvisioner adapts a users.Provisioner to auth.UserProvisioner.
func NewAuthProvisioner(p Provisioner) auth.UserProvisioner {
	return &authProvisionerAdapter{p: p}
}

func (a *authProvisionerAdapter) UpsertGoogleUser(ctx context.Context, googleID, email, name string, avatarURL *string, defaultRole string) (*auth.User, error) {
	u, err := a.p.ProvisionOAuthUser(ctx, googleID, email, name, avatarURL, defaultRole)
	if err != nil {
		return nil, err
	}
	return &auth.User{
		ID:           u.ID,
		Email:        u.Email,
		Name:         u.Name,
		GoogleID:     u.GoogleID,
		AvatarURL:    u.AvatarURL,
		Role:         u.Role,
		PhoneNumber:  u.PhoneNumber,
		IsActive:     u.IsActive,
		TokenVersion: u.TokenVersion,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}, nil
}

func (a *authProvisionerAdapter) SetRole(ctx context.Context, userID, role string) error {
	err := a.p.SetUserRole(ctx, userID, role)
	if errors.Is(err, ErrUserNotFound) {
		return auth.ErrUserNotFound
	}
	return err
}
