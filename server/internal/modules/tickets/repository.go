package tickets

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTicketNotFound = errors.New("ticket not found")
)

type Ticket struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	TicketCode  string     `json:"ticket_code"`
	IsCheckedIn bool       `json:"is_checked_in"`
	CheckedInAt *time.Time `json:"checked_in_at"`
	CheckedInBy *string    `json:"checked_in_by"`
	EmailSentAt *time.Time `json:"email_sent_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type TicketAttendeeInfo struct {
	TicketID        string
	UserID          string
	TicketCode      string
	IsCheckedIn     bool
	CheckedInAt     *time.Time
	CheckedInByID   *string
	CheckedInByName *string
	EmailSentAt     *time.Time
	UserName        string
	UserEmail       string
	UserRole        string
	UserPhone       *string
	TeamID          *string
	TeamName        *string
}

type EligibleUser struct {
	UserID      string
	Name        string
	Email       string
	TicketID    string
	TicketCode  string
	IsCheckedIn bool
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetOrCreateTicket inserts a ticket if not exists, or returns the existing ticket for the user.
func (r *Repository) GetOrCreateTicket(ctx context.Context, userID, ticketCode string) (*Ticket, error) {
	query := `
		INSERT INTO tickets (user_id, ticket_code)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
		RETURNING id, user_id, ticket_code, is_checked_in, checked_in_at, checked_in_by, email_sent_at, created_at, updated_at;
	`
	var t Ticket
	err := r.db.QueryRow(ctx, query, userID, ticketCode).Scan(
		&t.ID,
		&t.UserID,
		&t.TicketCode,
		&t.IsCheckedIn,
		&t.CheckedInAt,
		&t.CheckedInBy,
		&t.EmailSentAt,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get or create ticket: %w", err)
	}
	return &t, nil
}

// GetTicketByCode looks up a ticket and joins attendee profile and checker details.
func (r *Repository) GetTicketByCode(ctx context.Context, ticketCode string) (*TicketAttendeeInfo, error) {
	query := `
		SELECT 
			t.id, t.user_id, t.ticket_code, t.is_checked_in, t.checked_in_at, t.checked_in_by, t.email_sent_at,
			u.name, u.email, u.role::text, u.phone_number, u.team_id,
			tm.name,
			v.name
		FROM tickets t
		JOIN users u ON u.id = t.user_id AND u.deleted_at IS NULL
		LEFT JOIN teams tm ON tm.id = u.team_id
		LEFT JOIN users v ON v.id = t.checked_in_by
		WHERE t.ticket_code = $1;
	`
	var info TicketAttendeeInfo
	err := r.db.QueryRow(ctx, query, ticketCode).Scan(
		&info.TicketID,
		&info.UserID,
		&info.TicketCode,
		&info.IsCheckedIn,
		&info.CheckedInAt,
		&info.CheckedInByID,
		&info.EmailSentAt,
		&info.UserName,
		&info.UserEmail,
		&info.UserRole,
		&info.UserPhone,
		&info.TeamID,
		&info.TeamName,
		&info.CheckedInByName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTicketNotFound
		}
		return nil, fmt.Errorf("query ticket by code: %w", err)
	}
	return &info, nil
}

// GetTicketByUserID finds ticket details for an authenticated user.
func (r *Repository) GetTicketByUserID(ctx context.Context, userID string) (*Ticket, error) {
	query := `
		SELECT id, user_id, ticket_code, is_checked_in, checked_in_at, checked_in_by, email_sent_at, created_at, updated_at
		FROM tickets
		WHERE user_id = $1;
	`
	var t Ticket
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&t.ID,
		&t.UserID,
		&t.TicketCode,
		&t.IsCheckedIn,
		&t.CheckedInAt,
		&t.CheckedInBy,
		&t.EmailSentAt,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTicketNotFound
		}
		return nil, fmt.Errorf("query ticket by user id: %w", err)
	}
	return &t, nil
}

// MarkCheckedIn atomically marks a ticket as checked in. Returns checked_in_at timestamp.
// If already checked in or non-existent, returns false without modifying.
func (r *Repository) MarkCheckedIn(ctx context.Context, ticketID, volunteerID string) (*time.Time, bool, error) {
	query := `
		UPDATE tickets
		SET is_checked_in = true,
			checked_in_at = CURRENT_TIMESTAMP,
			checked_in_by = $2,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND is_checked_in = false
		RETURNING checked_in_at;
	`
	var checkedInAt time.Time
	err := r.db.QueryRow(ctx, query, ticketID, volunteerID).Scan(&checkedInAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("mark ticket checked in: %w", err)
	}
	return &checkedInAt, true, nil
}

// MarkEmailSent updates the email_sent_at timestamp.
func (r *Repository) MarkEmailSent(ctx context.Context, ticketID string) error {
	query := `UPDATE tickets SET email_sent_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = $1;`
	_, err := r.db.Exec(ctx, query, ticketID)
	if err != nil {
		return fmt.Errorf("mark email sent: %w", err)
	}
	return nil
}

// GetUserByID loads basic user info for single ticket dispatch.
func (r *Repository) GetUserByID(ctx context.Context, userID string) (string, string, error) {
	var name, email string
	query := `SELECT name, email FROM users WHERE id = $1 AND deleted_at IS NULL;`
	err := r.db.QueryRow(ctx, query, userID).Scan(&name, &email)
	if err != nil {
		return "", "", fmt.Errorf("get user for ticket dispatch: %w", err)
	}
	return name, email, nil
}

// FindEligibleUsers queries users based on bulk ticket send criteria.
func (r *Repository) FindEligibleUsers(ctx context.Context, req BulkSendTicketsRequest) ([]EligibleUser, error) {
	baseQuery := `
		SELECT 
			u.id, 
			u.name, 
			u.email, 
			COALESCE(t.id::text, ''), 
			COALESCE(t.ticket_code, ''), 
			COALESCE(t.is_checked_in, false)
		FROM users u
		LEFT JOIN tickets t ON t.user_id = u.id
		WHERE u.deleted_at IS NULL
	`
	args := []any{}
	argIdx := 1

	if len(req.UserIDs) > 0 {
		baseQuery += fmt.Sprintf(" AND u.id = ANY($%d)", argIdx)
		args = append(args, req.UserIDs)
		argIdx++
	}

	if req.TeamID != nil && *req.TeamID != "" {
		baseQuery += fmt.Sprintf(" AND u.team_id = $%d", argIdx)
		args = append(args, *req.TeamID)
		argIdx++
	}

	if req.Role != nil && *req.Role != "" {
		baseQuery += fmt.Sprintf(" AND u.role = $%d::user_role", argIdx)
		args = append(args, *req.Role)
		argIdx++
	}

	if req.SkipCheckedIn {
		baseQuery += " AND (t.is_checked_in IS NULL OR t.is_checked_in = false)"
	}

	baseQuery += " ORDER BY u.name ASC;"

	rows, err := r.db.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("find eligible users for tickets: %w", err)
	}
	defer rows.Close()

	var result []EligibleUser
	for rows.Next() {
		var u EligibleUser
		if err := rows.Scan(&u.UserID, &u.Name, &u.Email, &u.TicketID, &u.TicketCode, &u.IsCheckedIn); err != nil {
			return nil, fmt.Errorf("scan eligible user: %w", err)
		}
		result = append(result, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate eligible users: %w", err)
	}

	return result, nil
}

// GetAttendanceAnalytics calculates aggregate attendance metrics, hourly trends, team progress, and volunteer counts.
func (r *Repository) GetAttendanceAnalytics(ctx context.Context) (*AnalyticsResponse, error) {
	var totalIssued int
	var checkedInCount int

	// Totals
	countQuery := `
		SELECT 
			COUNT(*),
			COUNT(CASE WHEN is_checked_in THEN 1 END)
		FROM tickets;
	`
	err := r.db.QueryRow(ctx, countQuery).Scan(&totalIssued, &checkedInCount)
	if err != nil {
		return nil, fmt.Errorf("query ticket counts: %w", err)
	}

	var rate float64
	if totalIssued > 0 {
		rate = float64(checkedInCount) / float64(totalIssued) * 100.0
	}

	// Hourly arrivals
	hourlyQuery := `
		SELECT 
			to_char(date_trunc('hour', checked_in_at), 'YYYY-MM-DD HH24:00') AS hr,
			COUNT(*) AS cnt
		FROM tickets
		WHERE is_checked_in = true AND checked_in_at IS NOT NULL
		GROUP BY hr
		ORDER BY hr ASC;
	`
	hourlyRows, err := r.db.Query(ctx, hourlyQuery)
	if err != nil {
		return nil, fmt.Errorf("query hourly arrivals: %w", err)
	}
	defer hourlyRows.Close()

	var hourlyArrivals []HourlyArrival
	for hourlyRows.Next() {
		var ha HourlyArrival
		if err := hourlyRows.Scan(&ha.Hour, &ha.Count); err != nil {
			return nil, fmt.Errorf("scan hourly arrival: %w", err)
		}
		hourlyArrivals = append(hourlyArrivals, ha)
	}

	// Team completeness
	teamQuery := `
		SELECT 
			tm.id,
			tm.name,
			COUNT(u.id) AS total_members,
			COUNT(CASE WHEN t.is_checked_in THEN 1 END) AS checked_in_members
		FROM teams tm
		JOIN users u ON u.team_id = tm.id AND u.deleted_at IS NULL
		LEFT JOIN tickets t ON t.user_id = u.id
		GROUP BY tm.id, tm.name
		ORDER BY tm.name ASC;
	`
	teamRows, err := r.db.Query(ctx, teamQuery)
	if err != nil {
		return nil, fmt.Errorf("query team completeness: %w", err)
	}
	defer teamRows.Close()

	var teamSummaries []TeamCompletenessSummary
	for teamRows.Next() {
		var ts TeamCompletenessSummary
		if err := teamRows.Scan(&ts.TeamID, &ts.TeamName, &ts.TotalMembers, &ts.CheckedInMembers); err != nil {
			return nil, fmt.Errorf("scan team summary: %w", err)
		}

		if ts.TotalMembers > 0 && ts.CheckedInMembers == ts.TotalMembers {
			ts.Status = "complete"
		} else if ts.CheckedInMembers > 0 {
			ts.Status = "partial"
		} else {
			ts.Status = "absent"
		}

		teamSummaries = append(teamSummaries, ts)
	}

	if hourlyArrivals == nil {
		hourlyArrivals = []HourlyArrival{}
	}
	if teamSummaries == nil {
		teamSummaries = []TeamCompletenessSummary{}
	}

	return &AnalyticsResponse{
		TotalIssued:      totalIssued,
		CheckedInCount:   checkedInCount,
		AttendanceRate:   rate,
		HourlyArrivals:   hourlyArrivals,
		TeamCompleteness: teamSummaries,
	}, nil
}
