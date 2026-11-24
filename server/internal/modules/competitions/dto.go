package competitions

// CreateCompetitionRequest defines payload to create a new competition.
type CreateCompetitionRequest struct {
	Slug        string  `json:"slug" validate:"required,min=3,max=64"`
	Name        string  `json:"name" validate:"required,min=2,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=2000"`
	OwnerEmail  *string `json:"owner_email,omitempty" validate:"omitempty,email,max=254"`
	OwnerID     *string `json:"owner_id,omitempty" validate:"omitempty,uuid"`
}

// UpdateCompetitionRequest defines payload to update competition metadata.
type UpdateCompetitionRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=2,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=2000"`
}

// TransferOwnershipRequest defines payload to transfer competition ownership.
type TransferOwnershipRequest struct {
	NewOwnerUserID string `json:"new_owner_user_id" validate:"required,uuid"`
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
