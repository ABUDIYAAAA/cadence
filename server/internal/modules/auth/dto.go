package auth

import "time"

// LoginRequest defines credentials for logging in.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,min=8,max=128"`
}

// SetupPasswordRequest defines the payload for first-landing password setup.
type SetupPasswordRequest struct {
	Token           string `json:"token" validate:"required,max=256"`
	Password        string `json:"password" validate:"required,min=8,max=128"`
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
}

// ForgotPasswordRequest defines the payload to request a password reset link.
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email,max=254"`
}

// ResetPasswordRequest defines the payload to complete a password reset.
type ResetPasswordRequest struct {
	Token           string `json:"token" validate:"required,max=256"`
	Password        string `json:"password" validate:"required,min=8,max=128"`
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
}

// UserResponse represents the public user profile.
type UserResponse struct {
	ID                   string    `json:"id"`
	Email                string    `json:"email"`
	Name                 string    `json:"name"`
	PhoneNumber          *string   `json:"phone_number,omitempty"`
	Role                 string    `json:"role"`
	TeamID               *string   `json:"team_id,omitempty"`
	IsPasswordConfigured bool      `json:"is_password_configured"`
	CreatedAt            time.Time `json:"created_at"`
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
