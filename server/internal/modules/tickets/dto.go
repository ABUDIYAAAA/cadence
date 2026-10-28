package tickets

import "time"

// CheckInRequest defines the scanner payload containing the scanned QR ticket code.
type CheckInRequest struct {
	TicketCode string `json:"ticket_code" validate:"required"`
}

// AttendeeDetails encapsulates the profile of the scanned user.
type AttendeeDetails struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Email       string  `json:"email"`
	Role        string  `json:"role"`
	PhoneNumber *string `json:"phone_number,omitempty"`
	TeamID      *string `json:"team_id,omitempty"`
	TeamName    *string `json:"team_name,omitempty"`
}

// CheckInResponse returned on successful first-time check-in.
type CheckInResponse struct {
	Success     bool            `json:"success"`
	Message     string          `json:"message"`
	TicketCode  string          `json:"ticket_code"`
	CheckedInAt string          `json:"checked_in_at"`
	Attendee    AttendeeDetails `json:"attendee"`
}

// DuplicateCheckInResponse returned with HTTP 409 Conflict when a ticket was already used.
type DuplicateCheckInResponse struct {
	Error           string          `json:"error"`
	CheckedInAt     string          `json:"checked_in_at"`
	CheckedInByID   *string         `json:"checked_in_by_id,omitempty"`
	CheckedInByName *string         `json:"checked_in_by_name,omitempty"`
	Attendee        AttendeeDetails `json:"attendee"`
}

// BulkSendTicketsRequest filters recipients for asynchronous ticket distribution.
type BulkSendTicketsRequest struct {
	UserIDs       []string `json:"user_ids"`
	TeamID        *string  `json:"team_id"`
	Role          *string  `json:"role"`
	SkipCheckedIn bool     `json:"skip_checked_in"`
}

// BulkSendTicketsResponse summary of enqueued ticket jobs.
type BulkSendTicketsResponse struct {
	TotalProcessed int `json:"total_processed"`
	Enqueued       int `json:"enqueued"`
	Skipped        int `json:"skipped"`
}

// SingleSendTicketResponse returned when single user ticket is dispatched.
type SingleSendTicketResponse struct {
	Message    string `json:"message"`
	UserID     string `json:"user_id"`
	TicketCode string `json:"ticket_code"`
}

// MyTicketResponse returned to authenticated participants viewing their own entry pass.
type MyTicketResponse struct {
	TicketCode   string     `json:"ticket_code"`
	IsCheckedIn  bool       `json:"is_checked_in"`
	CheckedInAt  *time.Time `json:"checked_in_at,omitempty"`
	QRCodeBase64 string     `json:"qr_code_base64"`
}

// HourlyArrival represents aggregated check-ins per hour.
type HourlyArrival struct {
	Hour  string `json:"hour"`
	Count int    `json:"count"`
}

// TeamCompletenessSummary details team attendance completeness status.
type TeamCompletenessSummary struct {
	TeamID           string `json:"team_id"`
	TeamName         string `json:"team_name"`
	TotalMembers     int    `json:"total_members"`
	CheckedInMembers int    `json:"checked_in_members"`
	Status           string `json:"status"` // "complete", "partial", "absent"
}

// VolunteerLeaderboard summarizes check-ins performed per volunteer/admin.
type VolunteerLeaderboard struct {
	VolunteerID    string `json:"volunteer_id"`
	VolunteerName  string `json:"volunteer_name"`
	VolunteerEmail string `json:"volunteer_email"`
	Count          int    `json:"count"`
}

// AnalyticsResponse aggregates live attendance metrics.
type AnalyticsResponse struct {
	TotalIssued          int                       `json:"total_issued"`
	CheckedInCount       int                       `json:"checked_in_count"`
	AttendanceRate       float64                   `json:"attendance_rate"`
	HourlyArrivals       []HourlyArrival           `json:"hourly_arrivals"`
	TeamCompleteness     []TeamCompletenessSummary `json:"team_completeness"`
	VolunteerLeaderboard []VolunteerLeaderboard    `json:"volunteer_leaderboard"`
}
