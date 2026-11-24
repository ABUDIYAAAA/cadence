package competitions

import "time"

// CreateCompetitionRequest defines payload to create a new competition.
type CreateCompetitionRequest struct {
	Slug        string  `json:"slug" validate:"required,min=3,max=64"`
	Name        string  `json:"name" validate:"required,min=2,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=2000"`
	OwnerEmail  *string `json:"owner_email,omitempty" validate:"omitempty,email,max=254"`
	OwnerID     *string `json:"owner_id,omitempty" validate:"omitempty,uuid"`
	OwnerName   *string `json:"owner_name,omitempty" validate:"omitempty,max=255"`
}

// UpdateCompetitionRequest defines payload to update competition metadata.
type UpdateCompetitionRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=2,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=2000"`
}

// TransferOwnershipRequest defines payload to transfer competition ownership.
type TransferOwnershipRequest struct {
	NewOwnerUserID string  `json:"new_owner_user_id,omitempty" validate:"omitempty,uuid"`
	NewOwnerEmail  *string `json:"new_owner_email,omitempty" validate:"omitempty,email,max=254"`
	NewOwnerName   *string `json:"new_owner_name,omitempty" validate:"omitempty,max=255"`
}

// AssignMemberRoleRequest defines payload to assign or update a member's role.
type AssignMemberRoleRequest struct {
	UserID string `json:"user_id" validate:"required,uuid"`
	Role   string `json:"role" validate:"required,oneof=admin volunteer judge participant"`
}

// UpdateMemberRoleRequest defines payload to update a member's role.
type UpdateMemberRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=admin volunteer judge participant"`
}

// CompetitionResponse is the public response for a competition.
type CompetitionResponse struct {
	Competition
	Role *string `json:"my_role,omitempty"`
}

// MemberListResponse returns a list of members in a competition.
type MemberListResponse struct {
	Members []MemberWithUser `json:"members"`
	Total   int              `json:"total"`
}

// BanMemberRequest defines optional reason payload for banning a member.
type BanMemberRequest struct {
	Reason *string `json:"reason,omitempty" validate:"omitempty,max=500"`
}

// CompetitionUserFilter defines query parameters for paginated user listing within a competition.
type CompetitionUserFilter struct {
	Page          int     `json:"page"`
	Limit         int     `json:"limit"`
	Role          string  `json:"role"`
	TeamID        string  `json:"team_id"`
	NoTeam        bool    `json:"no_team"`
	Search        string  `json:"search"`
	IsBanned      *bool   `json:"is_banned"`
	CheckinStatus string  `json:"checkin_status"`
}

// CompetitionUserDetailResponse represents detailed user information scoped to a competition.
type CompetitionUserDetailResponse struct {
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

// CompetitionUserListResponse wraps paginated user list response in a competition.
type CompetitionUserListResponse struct {
	Users []CompetitionUserDetailResponse `json:"users"`
	Total int64                           `json:"total"`
	Page  int                             `json:"page"`
	Limit int                             `json:"limit"`
}
