package auth

import "time"

const (
	GlobalRoleDev  = "dev"
	GlobalRoleUser = "user"
)

// User represents the core account identity in the users table.
type User struct {
	ID           string
	Email        string
	Name         string
	GoogleID     *string
	AvatarURL    *string
	GlobalRole   string
	PhoneNumber  *string
	TeamID       *string
	IsActive     bool
	TokenVersion int
	CreatedAt    time.Time
	UpdatedAt    time.Time
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

// UserEventRole represents a role the user holds in a specific event.
type UserEventRole struct {
	EventID   string  `json:"event_id"`
	EventSlug string  `json:"event_slug"`
	EventName string  `json:"event_name"`
	LogoURL   *string `json:"logo_url,omitempty"`
	Role      string  `json:"role"`
}
