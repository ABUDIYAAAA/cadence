package teams

import "time"

// CreateTeamRequest defines the payload for creating a new team.
type CreateTeamRequest struct {
	Name             string   `json:"name" validate:"required,min=2,max=100"`
	LeaderID         *string  `json:"leader_id,omitempty"`
	InitialMemberIDs []string `json:"initial_member_ids,omitempty"`
}

// UpdateTeamRequest defines editable team properties.
type UpdateTeamRequest struct {
	Name     *string `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
	LeaderID *string `json:"leader_id,omitempty"`
}

// AddMemberRequest defines payload to add a user to a team.
type AddMemberRequest struct {
	UserID string `json:"user_id" validate:"required"`
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
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	LeaderID    *string   `json:"leader_id,omitempty"`
	LeaderName  *string   `json:"leader_name,omitempty"`
	LeaderEmail *string   `json:"leader_email,omitempty"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TeamDetailResponse represents the full team view with all members.
type TeamDetailResponse struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Leader    *TeamMemberSummary  `json:"leader,omitempty"`
	Members   []TeamMemberSummary `json:"members"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
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
