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

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, name, google_id, avatar_url, global_role, phone_number, team_id,
		       is_active, token_version, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`
	var u User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.Name, &u.GoogleID, &u.AvatarURL, &u.GlobalRole, &u.PhoneNumber, &u.TeamID,
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
		SELECT id, email, name, google_id, avatar_url, global_role, phone_number, team_id,
		       is_active, token_version, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.Name, &u.GoogleID, &u.AvatarURL, &u.GlobalRole, &u.PhoneNumber, &u.TeamID,
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
		SELECT id, email, name, google_id, avatar_url, global_role, phone_number, team_id,
		       is_active, token_version, created_at, updated_at
		FROM users
		WHERE google_id = $1
	`
	var u User
	err := r.db.QueryRow(ctx, query, googleID).Scan(
		&u.ID, &u.Email, &u.Name, &u.GoogleID, &u.AvatarURL, &u.GlobalRole, &u.PhoneNumber, &u.TeamID,
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

// UpsertGoogleUser creates or updates a user via Google OAuth identity.
func (r *Repository) UpsertGoogleUser(ctx context.Context, googleID, email, name string, avatarURL *string, defaultGlobalRole string) (*User, error) {
	if defaultGlobalRole == "" {
		defaultGlobalRole = GlobalRoleUser
	}

	query := `
		INSERT INTO users (email, name, google_id, avatar_url, global_role, is_active)
		VALUES (LOWER($1), $2, $3, $4, $5, TRUE)
		ON CONFLICT (email) DO UPDATE
		SET name = CASE WHEN users.name = '' THEN EXCLUDED.name ELSE users.name END,
		    google_id = COALESCE(users.google_id, EXCLUDED.google_id),
		    avatar_url = COALESCE(EXCLUDED.avatar_url, users.avatar_url),
		    global_role = CASE WHEN users.global_role = 'dev' THEN 'dev' ELSE EXCLUDED.global_role END,
		    is_active = TRUE,
		    deleted_at = NULL,
		    updated_at = CURRENT_TIMESTAMP
		RETURNING id, email, name, google_id, avatar_url, global_role, phone_number, team_id,
		          is_active, token_version, created_at, updated_at
	`
	var u User
	err := r.db.QueryRow(ctx, query, email, name, googleID, avatarURL, defaultGlobalRole).Scan(
		&u.ID, &u.Email, &u.Name, &u.GoogleID, &u.AvatarURL, &u.GlobalRole, &u.PhoneNumber, &u.TeamID,
		&u.IsActive, &u.TokenVersion, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert google user: %w", err)
	}

	// Link user to default competition
	compRole := "participant"
	if u.GlobalRole == GlobalRoleDev {
		compRole = "owner"
	}
	_, _ = r.db.Exec(ctx, `
		INSERT INTO user_competitions (user_id, competition_id, role)
		VALUES ($1, '00000000-0000-0000-0000-000000000001', $2::competition_role)
		ON CONFLICT (competition_id, user_id) DO NOTHING
	`, u.ID, compRole)

	return &u, nil
}

// GetUserCompetitions retrieves all competitions and associated roles for a user.
func (r *Repository) GetUserCompetitions(ctx context.Context, userID string) ([]UserCompetitionRole, error) {
	query := `
		SELECT c.id, c.slug, c.name, uc.role::text
		FROM user_competitions uc
		JOIN competitions c ON uc.competition_id = c.id
		WHERE uc.user_id = $1
		ORDER BY c.created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user competitions: %w", err)
	}
	defer rows.Close()

	comps := make([]UserCompetitionRole, 0)
	for rows.Next() {
		var item UserCompetitionRole
		if err := rows.Scan(&item.CompetitionID, &item.CompetitionSlug, &item.CompetitionName, &item.Role); err != nil {
			return nil, fmt.Errorf("scan user competition: %w", err)
		}
		comps = append(comps, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user competitions: %w", err)
	}
	return comps, nil
}

func (r *Repository) SetGlobalRole(ctx context.Context, userID, globalRole string) error {
	query := `
		UPDATE users
		SET global_role = $1, token_version = token_version + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`
	tag, err := r.db.Exec(ctx, query, globalRole, userID)
	if err != nil {
		return fmt.Errorf("set global role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
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
