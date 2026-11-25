package events

import "time"

const DefaultEventID = "00000000-0000-0000-0000-000000000001"

// Event represents a top-level event instance that encapsulates one or more competitions.
type Event struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	LogoURL     *string   `json:"logo_url,omitempty"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EventMember represents a user's membership, role, and ban status in an event.
type EventMember struct {
	ID        string     `json:"id"`
	EventID   string     `json:"event_id"`
	UserID    string     `json:"user_id"`
	Role      string     `json:"role"`
	IsBanned  bool       `json:"is_banned"`
	BannedAt  *time.Time `json:"banned_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// MemberWithUser joins event membership with user profile information.
type MemberWithUser struct {
	ID          string     `json:"id"`
	EventID     string     `json:"event_id"`
	UserID      string     `json:"user_id"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	PhoneNumber *string    `json:"phone_number,omitempty"`
	Role        string     `json:"role"`
	IsBanned    bool       `json:"is_banned"`
	BannedAt    *time.Time `json:"banned_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
