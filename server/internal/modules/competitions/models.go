package competitions

import "time"

const DefaultCompetitionID = "00000000-0000-0000-0000-000000000001"

// Competition represents a competition instance encapsulated within an Event.
type Competition struct {
	ID          string    `json:"id"`
	EventID     string    `json:"event_id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	MinTeamSize int       `json:"min_team_size"`
	MaxTeamSize int       `json:"max_team_size"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
