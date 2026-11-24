package competitions

import "time"

// Competition represents an event/hackathon instance.
type Competition struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CompetitionMember represents a user's membership and role in a competition.
type CompetitionMember struct {
	ID            string     `json:"id"`
	CompetitionID string     `json:"competition_id"`
	UserID        string     `json:"user_id"`
	Role          string     `json:"role"`
	IsBanned      bool       `json:"is_banned"`
	BannedAt      *time.Time `json:"banned_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// MemberWithUser joins competition membership with user profile information.
type MemberWithUser struct {
	ID            string     `json:"id"`
	CompetitionID string     `json:"competition_id"`
	UserID        string     `json:"user_id"`
	Email         string     `json:"email"`
	Name          string     `json:"name"`
	PhoneNumber   *string    `json:"phone_number,omitempty"`
	Role          string     `json:"role"`
	IsBanned      bool       `json:"is_banned"`
	BannedAt      *time.Time `json:"banned_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
