package teams

import "time"

// CreateTeamRequest is sent by a participant to start their own team.
type CreateTeamRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
}

// JoinTeamRequest carries the join code a teammate shared.
type JoinTeamRequest struct {
	Code string `json:"code" validate:"required,min=4,max=16"`
}

// RenameTeamRequest lets a team leader rename their own team.
type RenameTeamRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
}

// UpdateTeamRequest defines staff-editable team properties.
// MaxSize of 0 clears the per-team override.
type UpdateTeamRequest struct {
	Name     *string `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
	LeaderID *string `json:"leader_id,omitempty" validate:"omitempty,uuid"`
	MaxSize  *int    `json:"max_size,omitempty" validate:"omitempty,min=0,max=100"`
}

// AddMemberRequest defines payload for staff to add a participant to a team.
type AddMemberRequest struct {
	UserID string `json:"user_id" validate:"required,uuid"`
}

// UpdateSettingsRequest changes global team settings.
type UpdateSettingsRequest struct {
	MaxTeamSize *int `json:"max_team_size,omitempty" validate:"omitempty,min=1,max=100"`
}

// SettingsResponse describes global team settings.
type SettingsResponse struct {
	FrozenAll   bool `json:"frozen_all"`
	MaxTeamSize int  `json:"max_team_size"`
	// MaxTeamSizeSource is "database" once an admin saved a value, otherwise "environment".
	MaxTeamSizeSource string `json:"max_team_size_source"`
}

// TeamMemberSummary describes an individual participant within a team.
type TeamMemberSummary struct {
	ID                   string  `json:"id"`
	Email                string  `json:"email"`
	Name                 string  `json:"name"`
	PhoneNumber          *string `json:"phone_number,omitempty"`
	Role                 string  `json:"role"`
	IsLeader             bool    `json:"is_leader"`
	IsPasswordConfigured bool    `json:"is_password_configured"`
}

// TeamResponse represents the summary view of a team in listing endpoints.
type TeamResponse struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	JoinCode         string    `json:"join_code,omitempty"`
	LeaderID         *string   `json:"leader_id,omitempty"`
	LeaderName       *string   `json:"leader_name,omitempty"`
	LeaderEmail      *string   `json:"leader_email,omitempty"`
	MemberCount      int       `json:"member_count"`
	IsFrozen         bool      `json:"is_frozen"`
	EffectiveFrozen  bool      `json:"effective_frozen"`
	MaxSize          *int      `json:"max_size,omitempty"`
	EffectiveMaxSize int       `json:"effective_max_size"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TeamDetailResponse represents the full team view with all members.
type TeamDetailResponse struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	JoinCode         string              `json:"join_code,omitempty"`
	Leader           *TeamMemberSummary  `json:"leader,omitempty"`
	Members          []TeamMemberSummary `json:"members"`
	IsFrozen         bool                `json:"is_frozen"`
	EffectiveFrozen  bool                `json:"effective_frozen"`
	MaxSize          *int                `json:"max_size,omitempty"`
	EffectiveMaxSize int                 `json:"effective_max_size"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

// TeamFilter defines pagination and search options for teams.
type TeamFilter struct {
	Page   int    `json:"page"`
	Limit  int    `json:"limit"`
	Search string `json:"search"`
	// JudgeID restricts results to teams scheduled in panels this judge is assigned to.
	JudgeID *string `json:"-"`
}

// PaginatedTeamsResponse wraps team lists with metadata.
type PaginatedTeamsResponse struct {
	Data       []TeamResponse `json:"data"`
	TotalCount int64          `json:"total_count"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
	TotalPages int            `json:"total_pages"`
}
