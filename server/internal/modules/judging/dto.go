package judging

import "time"

// CreateCriteriaRequest defines a judging criterion within a round.
type CreateCriteriaRequest struct {
	Name        string  `json:"name" validate:"required,max=150"`
	Description string  `json:"description"`
	MinScore    int     `json:"min_score" validate:"min=0"`
	MaxScore    int     `json:"max_score" validate:"required,min=1"`
	Weight      float64 `json:"weight" validate:"required,min=0"`
	OrderIndex  int     `json:"order_index"`
}

// CreateRoundRequest creates a new judging round with criteria.
type CreateRoundRequest struct {
	Name             string                  `json:"name" validate:"required,max=255"`
	BufferSize       int                     `json:"buffer_size" validate:"min=1"`
	SlotDurationMins int                     `json:"slot_duration_mins" validate:"min=1"`
	Criteria         []CreateCriteriaRequest `json:"criteria" validate:"required,min=1,dive"`
}

// CreatePanelRequest creates a panel in a room and assigns judges.
type CreatePanelRequest struct {
	RoundID  string   `json:"round_id" validate:"required,uuid"`
	Name     string   `json:"name" validate:"required,max=150"`
	Room     string   `json:"room" validate:"required,max=100"`
	JudgeIDs []string `json:"judge_ids" validate:"required,min=1,dive,uuid"`
}

// AssignedJudge records a judge already seated on a panel within a round.
type AssignedJudge struct {
	JudgeID   string `json:"judge_id"`
	PanelID   string `json:"panel_id"`
	PanelName string `json:"panel_name"`
}

// SlotAssignment assigns a team to a slot sequence in a panel.
type SlotAssignment struct {
	TeamID          string            `json:"team_id" validate:"required,uuid"`
	SequenceOrder   int               `json:"sequence_order" validate:"required,min=1"`
	SubmissionLinks map[string]string `json:"submission_links"`
}

// ConfigureSlotsRequest configures sequential team slots for a panel.
type ConfigureSlotsRequest struct {
	Slots []SlotAssignment `json:"slots" validate:"required,min=1,dive"`
	// Append adds the teams after the panel's existing slots instead of replacing the
	// sequence, so teams can be distributed gradually as they are decided.
	Append bool `json:"append"`
}

// RoundPanelOverview summarises a panel and which teams it already holds.
type RoundPanelOverview struct {
	PanelID    string   `json:"panel_id"`
	Name       string   `json:"name"`
	Room       string   `json:"room"`
	JudgeCount int      `json:"judge_count"`
	SlotCount  int      `json:"slot_count"`
	TeamIDs    []string `json:"team_ids"`
	// Slots lists the panel's teams in pitch order, with the slot ids needed to move them.
	Slots []RoundPanelSlot `json:"slots"`
}

// RoundPanelSlot is a team's place in a panel's sequence.
type RoundPanelSlot struct {
	SlotID        string `json:"slot_id"`
	TeamID        string `json:"team_id"`
	TeamName      string `json:"team_name"`
	SequenceOrder int    `json:"sequence_order"`
	Status        string `json:"status"`
}

// UpdateSlotLinksRequest modifies project artifacts/links for a team's slot.
type UpdateSlotLinksRequest struct {
	SubmissionLinks map[string]string `json:"submission_links" validate:"required"`
}

// MoveSlotRequest payload for moving a team slot between panels or reordering in a queue.
type MoveSlotRequest struct {
	TargetPanelID       *string `json:"target_panel_id"`
	TargetSequenceOrder *int    `json:"target_sequence_order"`
	Status              *string `json:"status"` // e.g. "queued" to throw into room queue, or "scheduled"
}

// CriterionScoreInput captures a single judge criterion rating.
type CriterionScoreInput struct {
	CriterionID string  `json:"criterion_id" validate:"required,uuid"`
	Score       float64 `json:"score" validate:"min=0"`
	Notes       *string `json:"notes"`
}

// SubmitScoresRequest payload for a judge submitting/updating their scorecard.
type SubmitScoresRequest struct {
	Scores []CriterionScoreInput `json:"scores" validate:"required,min=1,dive"`
}

// PanelJudgeProgress is one panel judge's lock status for a slot. Scores are never exposed.
type PanelJudgeProgress struct {
	JudgeID  string `json:"judge_id"`
	Name     string `json:"name"`
	IsLocked bool   `json:"is_locked"`
}

// ExtendTimerRequest payload for adding time to a running pitch.
type ExtendTimerRequest struct {
	Seconds int `json:"seconds" validate:"required,min=30,max=900"`
}

// CriterionOutput representation of a criterion.
type CriterionOutput struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	MinScore    int     `json:"min_score"`
	MaxScore    int     `json:"max_score"`
	Weight      float64 `json:"weight"`
	OrderIndex  int     `json:"order_index"`
}

// CriterionScoreOutput holds a judge's score for a criterion.
type CriterionScoreOutput struct {
	CriterionID     string   `json:"criterion_id"`
	CriterionName   string   `json:"criterion_name"`
	MinScore        int      `json:"min_score"`
	MaxScore        int      `json:"max_score"`
	Weight          float64  `json:"weight"`
	Score           *float64 `json:"score"`
	NormalizedScore *float64 `json:"normalized_score,omitempty"`
	Notes           *string  `json:"notes"`
}

// JudgeSlotSummary lightweight slot item for judge queue overview.
type JudgeSlotSummary struct {
	SlotID               string     `json:"slot_id"`
	TeamID               string     `json:"team_id"`
	TeamName             string     `json:"team_name"`
	SequenceOrder        int        `json:"sequence_order"`
	Status               string     `json:"status"`
	EstimatedStartTime   *time.Time `json:"estimated_start_time,omitempty"`
	ActualStartTime      *time.Time `json:"actual_start_time,omitempty"`
	ActualEndTime        *time.Time `json:"actual_end_time,omitempty"`
	IsMyScorecardLocked  bool       `json:"is_my_scorecard_locked"`
	AllPanelJudgesLocked bool       `json:"all_panel_judges_locked"`
}

// JudgeSlotDetails full evaluation view for a judge evaluating a team.
// Includes timing counter info for frontend countdown timers.
type JudgeSlotDetails struct {
	SlotID              string     `json:"slot_id"`
	PanelID             string     `json:"panel_id"`
	PanelName           string     `json:"panel_name"`
	Room                string     `json:"room"`
	TeamID              string     `json:"team_id"`
	TeamName            string     `json:"team_name"`
	SequenceOrder       int        `json:"sequence_order"`
	Status              string     `json:"status"`
	SlotDurationMins    int        `json:"slot_duration_mins"`
	SlotDurationSeconds int        `json:"slot_duration_seconds"`
	ActualStartTime     *time.Time `json:"actual_start_time,omitempty"`
	TargetEndTime       *time.Time `json:"target_end_time,omitempty"`
	IsPaused            bool       `json:"is_paused"`
	PausedAt            *time.Time `json:"paused_at,omitempty"`
	PitchEnded          bool       `json:"pitch_ended"`
	PitchEndedAt        *time.Time `json:"pitch_ended_at,omitempty"`
	ExtraSeconds        int        `json:"extra_seconds"`
	// RemainingSeconds is the time left on the clock; negative once overtime. Set only while in progress.
	RemainingSeconds *int `json:"remaining_seconds,omitempty"`
	// ServerTime lets clients correct for clock skew when counting down to TargetEndTime.
	ServerTime           time.Time              `json:"server_time"`
	SubmissionLinks      map[string]string      `json:"submission_links"`
	CriteriaScores       []CriterionScoreOutput `json:"criteria_scores"`
	IsMyScorecardLocked  bool                   `json:"is_my_scorecard_locked"`
	AllPanelJudgesLocked bool                   `json:"all_panel_judges_locked"`
	LockedJudgesCount    int                    `json:"locked_judges_count"`
	TotalJudgesCount     int                    `json:"total_judges_count"`
	PanelJudges          []PanelJudgeProgress   `json:"panel_judges"`
}

// JudgePanelSummary summary of a panel assigned to the current judge.
type JudgePanelSummary struct {
	PanelID          string             `json:"panel_id"`
	PanelName        string             `json:"panel_name"`
	Room             string             `json:"room"`
	RoundID          string             `json:"round_id"`
	RoundName        string             `json:"round_name"`
	RoundStatus      string             `json:"round_status"`
	SlotDurationMins int                `json:"slot_duration_mins"`
	BufferSize       int                `json:"buffer_size"`
	Criteria         []CriterionOutput  `json:"criteria"`
	TotalSlots       int                `json:"total_slots"`
	CompletedSlots   int                `json:"completed_slots"`
	CurrentSlot      *JudgeSlotSummary  `json:"current_slot,omitempty"`
	Slots            []JudgeSlotSummary `json:"slots"`
}

// TeamStatusResponse participant view displaying live queue position and room coordinates.
type TeamStatusResponse struct {
	HasActiveRound        bool              `json:"has_active_round"`
	RoundName             string            `json:"round_name,omitempty"`
	PanelName             string            `json:"panel_name,omitempty"`
	Room                  string            `json:"room,omitempty"`
	Status                string            `json:"status,omitempty"` // "scheduled", "queued", "in_progress", "completed"
	QueuePosition         int               `json:"queue_position,omitempty"`
	EstimatedStartTime    *time.Time        `json:"estimated_start_time,omitempty"`
	EstimatedDurationMins int               `json:"estimated_duration_mins,omitempty"`
	Instructions          string            `json:"instructions,omitempty"`
	SubmissionLinks       map[string]string `json:"submission_links,omitempty"`
}

// TeamSlotSummary used by volunteers to identify and escort teams.
type TeamSlotSummary struct {
	SlotID             string            `json:"slot_id"`
	TeamID             string            `json:"team_id"`
	TeamName           string            `json:"team_name"`
	SequenceOrder      int               `json:"sequence_order"`
	Status             string            `json:"status"`
	EstimatedStartTime *time.Time        `json:"estimated_start_time,omitempty"`
	ActualStartTime    *time.Time        `json:"actual_start_time,omitempty"`
	SubmissionLinks    map[string]string `json:"submission_links,omitempty"`
}

// VolunteerPanelSchedule coordinator view per room.
type VolunteerPanelSchedule struct {
	PanelID          string            `json:"panel_id"`
	PanelName        string            `json:"panel_name"`
	Room             string            `json:"room"`
	RoundID          string            `json:"round_id"`
	RoundName        string            `json:"round_name"`
	SlotDurationMins int               `json:"slot_duration_mins"`
	BufferSize       int               `json:"buffer_size"`
	ActiveTeam       *TeamSlotSummary  `json:"active_team,omitempty"`
	QueuedTeams      []TeamSlotSummary `json:"queued_teams"`
	UpcomingTeams    []TeamSlotSummary `json:"upcoming_teams"`
	CompletedCount   int               `json:"completed_count"`
	TotalCount       int               `json:"total_count"`
}

// AdminLeaderboardEntry ranking row for admin eyes only.
type AdminLeaderboardEntry struct {
	Rank                   int                `json:"rank"`
	TeamID                 string             `json:"team_id"`
	TeamName               string             `json:"team_name"`
	PanelName              string             `json:"panel_name"`
	Room                   string             `json:"room"`
	SlotStatus             string             `json:"slot_status"`
	CriteriaAverages       map[string]float64 `json:"criteria_averages"`
	CriteriaWeightedScores map[string]float64 `json:"criteria_weighted_scores,omitempty"`
	TotalWeightedScore     float64            `json:"total_weighted_score"`
	MaxPossibleScore       float64            `json:"max_possible_score"`
	AverageRawScore        float64            `json:"average_raw_score"`
	AverageScore           float64            `json:"average_score"`
	JudgesEvaluatedCount   int                `json:"judges_evaluated_count"`
	TotalJudgesOnPanel     int                `json:"total_judges_on_panel"`
	AllPanelJudgesLocked   bool               `json:"all_panel_judges_locked"`
}

// AdminLeaderboardResponse complete round leaderboard for administrators.
type AdminLeaderboardResponse struct {
	RoundID          string                  `json:"round_id"`
	RoundName        string                  `json:"round_name"`
	Status           string                  `json:"status"`
	Criteria         []CriterionOutput       `json:"criteria"`
	Leaderboard      []AdminLeaderboardEntry `json:"leaderboard"`
	TotalTeams       int                     `json:"total_teams"`
	GradedTeams      int                     `json:"graded_teams"`
	MaxPossibleScore float64                 `json:"max_possible_score"`
	AverageScore     float64                 `json:"average_score"`
}
