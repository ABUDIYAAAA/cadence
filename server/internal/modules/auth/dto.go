package auth

import "time"

// GoogleAuthURLResponse returns the OAuth authorization URL for the client.
type GoogleAuthURLResponse struct {
	URL string `json:"url"`
}

// UserResponse represents the public user profile.
type UserResponse struct {
	ID          string          `json:"id"`
	Email       string          `json:"email"`
	Name        string          `json:"name"`
	PhoneNumber *string         `json:"phone_number,omitempty"`
	GoogleID    *string         `json:"google_id,omitempty"`
	AvatarURL   *string         `json:"avatar_url,omitempty"`
	Role        string          `json:"role,omitempty"`
	TeamID      *string         `json:"team_id,omitempty"`
	Events      []UserEventRole `json:"events,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// AuthResponse is returned on successful authentication.
type AuthResponse struct {
	User    UserResponse `json:"user"`
	Message string       `json:"message"`
}

// MessageResponse represents simple status responses.
type MessageResponse struct {
	Message string `json:"message"`
}
