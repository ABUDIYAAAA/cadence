package users

import "time"

// CreateUserRequest defines payload for single or bulk user creation.
type CreateUserRequest struct {
	Email       string  `json:"email" validate:"required,email,max=254"`
	Name        string  `json:"name" validate:"required,max=255"`
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
	// CheckinStatus narrows to "checked_in", "not_checked_in" (has a pass, not yet arrived) or "no_pass".
	CheckinStatus string `json:"checkin_status"`
}

// BulkInviteRequest defines target users for onboarding emails.
type BulkInviteRequest struct {
	UserIDs      []string `json:"user_ids"`
	AllUninvited bool     `json:"all_uninvited"`
	// ExcludeDomains lists email domains that must not receive an invite.
	ExcludeDomains []string `json:"exclude_domains"`
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
	TokenVersion         int       `json:"token_version"`
	CreatedAt            time.Time `json:"created_at"`
	// Set only by the list endpoint: "checked_in", "not_checked_in" or "no_pass".
	CheckinStatus string     `json:"checkin_status,omitempty"`
	CheckedInAt   *time.Time `json:"checked_in_at,omitempty"`
}

// PaginatedUsersResponse wraps user lists with pagination metadata.
type PaginatedUsersResponse struct {
	Data       []UserDetailResponse `json:"data"`
	TotalCount int64                `json:"total_count"`
	Page       int                  `json:"page"`
	Limit      int                  `json:"limit"`
	TotalPages int                  `json:"total_pages"`
}
