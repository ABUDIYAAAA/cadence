package users

import "time"

// CreateUserRequest defines payload for single or bulk user creation.
type CreateUserRequest struct {
	Email       string  `json:"email" validate:"required,email"`
	Name        string  `json:"name" validate:"required"`
	Role        string  `json:"role" validate:"omitempty,oneof=admin judge volunteer participant"`
	PhoneNumber *string `json:"phone_number,omitempty"`
	TeamName    *string `json:"team_name,omitempty"`
	TeamID      *string `json:"team_id,omitempty"`
	IsLeader    bool    `json:"is_leader"`
	SendInvite  bool    `json:"send_invite"`
}

// BulkCreateUsersRequest encapsulates multiple user creations parsed from frontend CSV.
type BulkCreateUsersRequest struct {
	Users []CreateUserRequest `json:"users" validate:"required,min=1,dive"`
}

// BulkCreateResponse reports the outcome of a bulk import operation.
type BulkCreateResponse struct {
	TotalProcessed    int                  `json:"total_processed"`
	TotalCreated      int                  `json:"total_created"`
	SkippedCount      int                  `json:"skipped_count"`
	SkippedErrors     []string             `json:"skipped_errors,omitempty"`
	InvitesDispatched int                  `json:"invites_dispatched"`
	Users             []UserDetailResponse `json:"users"`
}

// UpdateUserRequest defines editable user profile properties.
type UpdateUserRequest struct {
	Name        *string `json:"name,omitempty"`
	PhoneNumber *string `json:"phone_number,omitempty"`
	Role        *string `json:"role,omitempty" validate:"omitempty,oneof=admin judge volunteer participant"`
	TeamID      *string `json:"team_id,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}

// UserFilter defines query parameters for paginated user listing.
type UserFilter struct {
	Page     int    `json:"page"`
	Limit    int    `json:"limit"`
	Role     string `json:"role"`
	TeamID   string `json:"team_id"`
	Search   string `json:"search"`
	IsActive *bool  `json:"is_active"`
}

// BulkInviteRequest defines target users for onboarding emails.
type BulkInviteRequest struct {
	UserIDs      []string `json:"user_ids"`
	AllUninvited bool     `json:"all_uninvited"`
}

// UserDetailResponse is the complete user view returned to administrators.
type UserDetailResponse struct {
	ID                   string    `json:"id"`
	Email                string    `json:"email"`
	Name                 string    `json:"name"`
	PhoneNumber          *string   `json:"phone_number,omitempty"`
	Role                 string    `json:"role"`
	TeamID               *string   `json:"team_id,omitempty"`
	TeamName             *string   `json:"team_name,omitempty"`
	IsLeader             bool      `json:"is_leader"`
	IsPasswordConfigured bool      `json:"is_password_configured"`
	IsActive             bool      `json:"is_active"`
	CreatedAt            time.Time `json:"created_at"`
}

// PaginatedUsersResponse wraps user lists with pagination metadata.
type PaginatedUsersResponse struct {
	Data       []UserDetailResponse `json:"data"`
	TotalCount int64                `json:"total_count"`
	Page       int                  `json:"page"`
	Limit      int                  `json:"limit"`
	TotalPages int                  `json:"total_pages"`
}
