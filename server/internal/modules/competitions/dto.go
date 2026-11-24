package competitions

// CreateCompetitionRequest defines payload to create a new competition inside an event.
type CreateCompetitionRequest struct {
	Slug        string  `json:"slug" validate:"required,min=2,max=64"`
	Name        string  `json:"name" validate:"required,min=2,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=2000"`
}

// UpdateCompetitionRequest defines payload to update competition metadata.
type UpdateCompetitionRequest struct {
	Slug        *string `json:"slug,omitempty" validate:"omitempty,min=2,max=64"`
	Name        *string `json:"name,omitempty" validate:"omitempty,min=2,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=2000"`
}

// CompetitionResponse is the response for a competition along with the caller's role in the parent event.
type CompetitionResponse struct {
	Competition
	Role *string `json:"my_role,omitempty"`
}

// CompetitionListResponse wraps a list of competitions in an event.
type CompetitionListResponse struct {
	Competitions []Competition `json:"competitions"`
	Total        int           `json:"total"`
}
