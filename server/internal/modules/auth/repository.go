package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrTokenNotFound      = errors.New("token not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type User struct {
	ID                   string
	Email                string
	PasswordHash         *string
	Name                 string
	PhoneNumber          *string
	Role                 string
	TeamID               *string
	IsPasswordConfigured bool
	IsActive             bool
	TokenVersion         int
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type RefreshToken struct {
	ID        string
	UserID    string
	DeviceID  string
	TokenHash string
	FamilyID  string
	ParentID  *string
	UserAgent *string
	IPAddress *string
	IsRevoked bool
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time
}

type ResetToken struct {
	ID        string
	UserID    string
	TokenHash string
	Purpose   string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, password_hash, name, phone_number, role, team_id,
		       is_password_configured, is_active, token_version, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`
	var u User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.PhoneNumber, &u.Role, &u.TeamID,
		&u.IsPasswordConfigured, &u.IsActive, &u.TokenVersion, &u.CreatedAt, &u.UpdatedAt,
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
		SELECT id, email, password_hash, name, phone_number, role, team_id,
		       is_password_configured, is_active, token_version, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.PhoneNumber, &u.Role, &u.TeamID,
		&u.IsPasswordConfigured, &u.IsActive, &u.TokenVersion, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (r *Repository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	query := `
		UPDATE users
		SET password_hash = $1, is_password_configured = TRUE, token_version = token_version + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`
	tag, err := r.db.Exec(ctx, query, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *Repository) CreateRefreshToken(ctx context.Context, s *RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (user_id, device_id, token_hash, family_id, parent_id, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query,
		s.UserID, s.DeviceID, s.TokenHash, s.FamilyID, s.ParentID, s.UserAgent, s.IPAddress, s.ExpiresAt,
	).Scan(&s.ID, &s.CreatedAt)
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

func (r *Repository) CreateResetToken(ctx context.Context, t *ResetToken) error {
	query := `
		INSERT INTO password_reset_tokens (user_id, token_hash, purpose, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query, t.UserID, t.TokenHash, t.Purpose, t.ExpiresAt).Scan(&t.ID, &t.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert reset token: %w", err)
	}
	return nil
}

func (r *Repository) GetResetTokenByHash(ctx context.Context, tokenHash, purpose string) (*ResetToken, error) {
	query := `
		SELECT id, user_id, token_hash, purpose, expires_at, used_at, created_at
		FROM password_reset_tokens
		WHERE token_hash = $1 AND purpose = $2
	`
	var t ResetToken
	err := r.db.QueryRow(ctx, query, tokenHash, purpose).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.Purpose, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("get reset token by hash: %w", err)
	}
	return &t, nil
}

func (r *Repository) MarkResetTokenUsed(ctx context.Context, id string) error {
	query := `
		UPDATE password_reset_tokens
		SET used_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark reset token used: %w", err)
	}
	return nil
}
