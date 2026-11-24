package events

import "time"

// CreateEventRequest defines payload to create a new event (restricted to devs).
type CreateEventRequest struct {
	Slug        string  `json:"slug" validate:"required,min=2,max=64"`
	Name        string  `json:"name" validate:"required,min=2,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=2000"`
	LogoURL     *string `json:"logo_url,omitempty"`
	PhotoURL    *string `json:"photo_url,omitempty"`
	OwnerEmail  *string `json:"owner_email,omitempty" validate:"omitempty,email,max=254"`
	OwnerID     *string `json:"owner_id,omitempty" validate:"omitempty,uuid"`
	OwnerName   *string `json:"owner_name,omitempty" validate:"omitempty,max=255"`
}

// UpdateEventRequest defines payload to update event metadata.
type UpdateEventRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=2,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=2000"`
	LogoURL     *string `json:"logo_url,omitempty"`
	PhotoURL    *string `json:"photo_url,omitempty"`
}

// TransferOwnershipRequest defines payload to transfer event ownership.
type TransferOwnershipRequest struct {
	NewOwnerUserID string  `json:"new_owner_user_id,omitempty" validate:"omitempty,uuid"`
	NewOwnerEmail  *string `json:"new_owner_email,omitempty" validate:"omitempty,email,max=254"`
	NewOwnerName   *string `json:"new_owner_name,omitempty" validate:"omitempty,max=255"`
}

// AssignMemberRoleRequest defines payload to assign or update a member's role in an event.
type AssignMemberRoleRequest struct {
	UserID *string `json:"user_id,omitempty" validate:"omitempty,uuid"`
	Email  *string `json:"email,omitempty" validate:"omitempty,email,max=254"`
	Name   *string `json:"name,omitempty" validate:"omitempty,max=255"`
	Role   string  `json:"role" validate:"required,oneof=admin volunteer judge participant"`
}

// AddEventUserRequest defines payload to add a single user to an event by email.
type AddEventUserRequest struct {
	Email       string  `json:"email" validate:"required,email,max=254"`
	Name        *string `json:"name,omitempty" validate:"omitempty,max=255"`
	Role        string  `json:"role" validate:"omitempty,oneof=admin volunteer judge participant"`
	PhoneNumber *string `json:"phone_number,omitempty" validate:"omitempty,max=32"`
}

// BulkAddEventUsersRequest defines payload to bulk add users to an event via emails.
type BulkAddEventUsersRequest struct {
	Users []AddEventUserRequest `json:"users" validate:"required,min=1,dive"`
}

// BulkAddEventUsersResponse reports outcome of a bulk user add operation in an event.
type BulkAddEventUsersResponse struct {
	TotalProcessed int                       `json:"total_processed"`
	TotalCreated   int                       `json:"total_created"`
	TotalAdded     int                       `json:"total_added"`
	SkippedCount   int                       `json:"skipped_count"`
	SkippedErrors  []string                  `json:"skipped_errors,omitempty"`
	Users          []EventUserDetailResponse `json:"users"`
}

// UpdateMemberRoleRequest defines payload to update a member's role.
type UpdateMemberRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=admin volunteer judge participant"`
}

// EventResponse is the public response for an event.
type EventResponse struct {
	Event
	Role *string `json:"my_role,omitempty"`
}

// MemberListResponse returns a list of members in an event.
type MemberListResponse struct {
	Members []MemberWithUser `json:"members"`
	Total   int              `json:"total"`
}

// BanMemberRequest defines optional reason payload for banning a member from an event.
type BanMemberRequest struct {
	Reason *string `json:"reason,omitempty" validate:"omitempty,max=500"`
}

// EventUserFilter defines query parameters for paginated user listing within an event.
type EventUserFilter struct {
	Page          int    `json:"page"`
	Limit         int    `json:"limit"`
	Role          string `json:"role"`
	CompetitionID string `json:"competition_id"`
	TeamID        string `json:"team_id"`
	NoTeam        bool   `json:"no_team"`
	Search        string `json:"search"`
	IsBanned      *bool  `json:"is_banned"`
	CheckinStatus string `json:"checkin_status"`
}

// EventUserDetailResponse represents detailed user information scoped to an event.
type EventUserDetailResponse struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	Name          string     `json:"name"`
	PhoneNumber   *string    `json:"phone_number,omitempty"`
	GlobalRole    string     `json:"global_role"`
	AvatarURL     *string    `json:"avatar_url,omitempty"`
	Role          string     `json:"role"`
	IsBanned      bool       `json:"is_banned"`
	BannedAt      *time.Time `json:"banned_at,omitempty"`
	TeamID        *string    `json:"team_id,omitempty"`
	TeamName      *string    `json:"team_name,omitempty"`
	IsLeader      *bool      `json:"is_leader,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	CheckinStatus string     `json:"checkin_status"`
	CheckedInAt   *time.Time `json:"checked_in_at,omitempty"`
}

// EventUserListResponse wraps paginated user list response in an event.
type EventUserListResponse struct {
	Users []EventUserDetailResponse `json:"users"`
	Total int64                     `json:"total"`
	Page  int                       `json:"page"`
	Limit int                       `json:"limit"`
}
