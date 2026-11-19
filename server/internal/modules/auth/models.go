package auth

import "time"

// User represents the core account identity in the users table.
type User struct {
	ID                   string
	Email                string
	PasswordHash         *string
	Name                 string
	PhoneNumber          *string
	TeamID               *string
	IsPasswordConfigured bool
	IsActive             bool
	TokenVersion         int
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// RefreshToken represents a long-lived user session bound to a device.
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

// ResetToken represents a one-time token for initial setup or password reset.
type ResetToken struct {
	ID        string
	UserID    string
	TokenHash string
	Purpose   string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// UserCompetitionRole represents a role the user holds in a specific competition.
type UserCompetitionRole struct {
	CompetitionID   string `json:"competition_id"`
	CompetitionSlug string `json:"competition_slug"`
	CompetitionName string `json:"competition_name"`
	Role            string `json:"role"`
}
