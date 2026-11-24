package tickets

import (
	"context"
	"errors"
	"fmt"
	"time"

	"devclubxnst.com/cadence/internal/modules/events"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	EmailStatusPending       = "pending"
	EmailStatusQueued        = "queued"
	EmailStatusSent          = "sent"
	EmailStatusFailed        = "failed"
	EmailStatusSkippedDomain = "skipped_domain"
)

var (
	ErrTicketNotFound = errors.New("ticket not found")
)

type Ticket struct {
	ID            string     `json:"id"`
	EventID       string     `json:"event_id"`
	UserID        string     `json:"user_id"`
	TicketCode    string     `json:"ticket_code"`
	IsCheckedIn   bool       `json:"is_checked_in"`
	CheckedInAt   *time.Time `json:"checked_in_at"`
	CheckedInBy   *string    `json:"checked_in_by"`
	EmailStatus   string     `json:"email_status"`
	EmailQueuedAt *time.Time `json:"email_queued_at,omitempty"`
	EmailSentAt   *time.Time `json:"email_sent_at"`
	EmailError    *string    `json:"email_error,omitempty"`
	EmailAttempts int        `json:"email_attempts"`
	ReleasedAt    *time.Time `json:"released_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type TicketAttendeeInfo struct {
	TicketID        string
	EventID         string
	UserID          string
	TicketCode      string
	IsCheckedIn     bool
	CheckedInAt     *time.Time
	CheckedInByID   *string
	CheckedInByName *string
	EmailStatus     string
	EmailSentAt     *time.Time
	ReleasedAt      *time.Time
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
	EmailStatus string
	EmailSentAt *time.Time
	ReleasedAt  *time.Time
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func resolveEventID(ctx context.Context) string {
	if evCtx, ok := events.FromContext(ctx); ok && evCtx != nil && evCtx.EventID != "" {
		return evCtx.EventID
	}
	if ev, ok := events.GetEvent(ctx); ok && ev != nil && ev.ID != "" {
		return ev.ID
	}
	return events.DefaultEventID
}

const ticketColumns = `id, event_id, user_id, ticket_code, is_checked_in, checked_in_at, checked_in_by,
	email_status, email_queued_at, email_sent_at, email_error, email_attempts, released_at, created_at, updated_at`

func scanTicket(row pgx.Row, t *Ticket) error {
	return row.Scan(
		&t.ID,
		&t.EventID,
		&t.UserID,
		&t.TicketCode,
		&t.IsCheckedIn,
		&t.CheckedInAt,
		&t.CheckedInBy,
		&t.EmailStatus,
		&t.EmailQueuedAt,
		&t.EmailSentAt,
		&t.EmailError,
		&t.EmailAttempts,
		&t.ReleasedAt,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
}

// GetOrCreateTicket inserts a ticket if not exists, or returns the existing ticket for the user in the active event.
func (r *Repository) GetOrCreateTicket(ctx context.Context, userID, ticketCode string) (*Ticket, error) {
	eventID := resolveEventID(ctx)
	query := fmt.Sprintf(`
		INSERT INTO tickets (event_id, user_id, ticket_code)
		VALUES ($1, $2, $3)
		ON CONFLICT (event_id, user_id) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
		RETURNING %s;
	`, ticketColumns)
	var t Ticket
	if err := scanTicket(r.db.QueryRow(ctx, query, eventID, userID, ticketCode), &t); err != nil {
		return nil, fmt.Errorf("get or create ticket: %w", err)
	}
	return &t, nil
}

// GetOrCreateReleasedTicket inserts a ticket (or updates an existing one) and guarantees released_at is set.
func (r *Repository) GetOrCreateReleasedTicket(ctx context.Context, userID, ticketCode string) (*Ticket, error) {
	eventID := resolveEventID(ctx)
	query := fmt.Sprintf(`
		INSERT INTO tickets (event_id, user_id, ticket_code, released_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (event_id, user_id) DO UPDATE
		SET released_at = COALESCE(tickets.released_at, CURRENT_TIMESTAMP),
		    updated_at = CURRENT_TIMESTAMP
		RETURNING %s;
	`, ticketColumns)
	var t Ticket
	if err := scanTicket(r.db.QueryRow(ctx, query, eventID, userID, ticketCode), &t); err != nil {
		return nil, fmt.Errorf("get or create released ticket: %w", err)
	}
	return &t, nil
}

// IsEventMember checks whether userID is an active, non-banned member of the active event (or a global dev).
func (r *Repository) IsEventMember(ctx context.Context, userID string) (bool, error) {
	eventID := resolveEventID(ctx)
	var allowed bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users u
			LEFT JOIN user_events ue ON ue.user_id = u.id AND ue.event_id = $1
			WHERE u.id = $2
			  AND u.deleted_at IS NULL
			  AND u.is_active = TRUE
			  AND (u.global_role = 'dev' OR (ue.user_id IS NOT NULL AND ue.is_banned = FALSE))
		)
	`, eventID, userID).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("check event member: %w", err)
	}
	return allowed, nil
}

// GetTicketByCode looks up a ticket in the active event and joins attendee profile and checker details.
func (r *Repository) GetTicketByCode(ctx context.Context, ticketCode string) (*TicketAttendeeInfo, error) {
	eventID := resolveEventID(ctx)
	query := `
		SELECT 
			t.id, t.event_id, t.user_id, t.ticket_code, t.is_checked_in, t.checked_in_at, t.checked_in_by,
			t.email_status, t.email_sent_at, t.released_at,
			u.name, u.email, COALESCE(ue.role::text, 'participant'), u.phone_number,
			tm_sub.team_id, tm_sub.team_name,
			v.name
		FROM tickets t
		JOIN users u ON u.id = t.user_id AND u.deleted_at IS NULL
		LEFT JOIN user_events ue ON ue.user_id = u.id AND ue.event_id = t.event_id
		LEFT JOIN LATERAL (
			SELECT tm.team_id, tm_t.name AS team_name
			FROM team_members tm
			JOIN teams tm_t ON tm_t.id = tm.team_id
			JOIN competitions c ON c.id = tm_t.competition_id
			WHERE tm.user_id = u.id AND c.event_id = t.event_id
			ORDER BY tm.joined_at DESC
			LIMIT 1
		) tm_sub ON TRUE
		LEFT JOIN users v ON v.id = t.checked_in_by
		WHERE t.event_id = $1 AND t.ticket_code = $2;
	`
	var info TicketAttendeeInfo
	err := r.db.QueryRow(ctx, query, eventID, ticketCode).Scan(
		&info.TicketID,
		&info.EventID,
		&info.UserID,
		&info.TicketCode,
		&info.IsCheckedIn,
		&info.CheckedInAt,
		&info.CheckedInByID,
		&info.EmailStatus,
		&info.EmailSentAt,
		&info.ReleasedAt,
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

// GetTicketByUserID finds ticket details for an authenticated user in the active event.
func (r *Repository) GetTicketByUserID(ctx context.Context, userID string) (*Ticket, error) {
	eventID := resolveEventID(ctx)
	query := fmt.Sprintf(`
		SELECT %s
		FROM tickets
		WHERE event_id = $1 AND user_id = $2;
	`, ticketColumns)
	var t Ticket
	if err := scanTicket(r.db.QueryRow(ctx, query, eventID, userID), &t); err != nil {
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

// ClaimTicketForEmail atomically transitions a ticket into 'queued' state prior to publishing to RabbitMQ.
// If forceResend is false, it refuses to claim tickets that are already 'queued' or 'sent'.
func (r *Repository) ClaimTicketForEmail(ctx context.Context, ticketID string, forceResend bool) (bool, error) {
	query := `
		UPDATE tickets
		SET email_status = 'queued',
		    email_queued_at = CURRENT_TIMESTAMP,
		    email_attempts = email_attempts + 1,
		    email_error = NULL,
		    released_at = COALESCE(released_at, CURRENT_TIMESTAMP),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
		  AND ($2 = TRUE OR (email_status NOT IN ('queued', 'sent') AND email_sent_at IS NULL))
		RETURNING id;
	`
	var claimedID string
	err := r.db.QueryRow(ctx, query, ticketID, forceResend).Scan(&claimedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("claim ticket for email: %w", err)
	}
	return true, nil
}

// MarkTicketDomainExcluded records that a ticket holder's domain is excluded from QR email dispatch
// while keeping their pass released on the dashboard.
func (r *Repository) MarkTicketDomainExcluded(ctx context.Context, ticketID string) error {
	query := `
		UPDATE tickets
		SET email_status = CASE WHEN email_status = 'sent' THEN 'sent' ELSE 'skipped_domain' END,
		    released_at = COALESCE(released_at, CURRENT_TIMESTAMP),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1;
	`
	if _, err := r.db.Exec(ctx, query, ticketID); err != nil {
		return fmt.Errorf("mark ticket domain excluded: %w", err)
	}
	return nil
}

// MarkTicketEmailDelivered marks a ticket email as delivered via SMTP.
func (r *Repository) MarkTicketEmailDelivered(ctx context.Context, ticketID string) error {
	query := `
		UPDATE tickets
		SET email_status = 'sent',
		    email_sent_at = CURRENT_TIMESTAMP,
		    email_error = NULL,
		    released_at = COALESCE(released_at, CURRENT_TIMESTAMP),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1;
	`
	if _, err := r.db.Exec(ctx, query, ticketID); err != nil {
		return fmt.Errorf("mark ticket email delivered: %w", err)
	}
	return nil
}

// MarkTicketEmailFailed records a permanent or enqueue failure on a ticket so future bulk sends can retry it.
func (r *Repository) MarkTicketEmailFailed(ctx context.Context, ticketID, errMsg string) error {
	query := `
		UPDATE tickets
		SET email_status = 'failed',
		    email_error = $2,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1;
	`
	if _, err := r.db.Exec(ctx, query, ticketID, errMsg); err != nil {
		return fmt.Errorf("mark ticket email failed: %w", err)
	}
	return nil
}

// MarkEmailSent updates the email_sent_at and email_status='sent' timestamp.
func (r *Repository) MarkEmailSent(ctx context.Context, ticketID string) error {
	return r.MarkTicketEmailDelivered(ctx, ticketID)
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

// FindEligibleUsers queries users in the active event based on bulk ticket send criteria.
func (r *Repository) FindEligibleUsers(ctx context.Context, req BulkSendTicketsRequest) ([]EligibleUser, error) {
	eventID := resolveEventID(ctx)
	baseQuery := `
		SELECT 
			u.id, 
			u.name, 
			u.email, 
			COALESCE(t.id::text, ''), 
			COALESCE(t.ticket_code, ''), 
			COALESCE(t.is_checked_in, false),
			COALESCE(t.email_status, 'pending'),
			t.email_sent_at,
			t.released_at
		FROM user_events ue
		JOIN users u ON u.id = ue.user_id
		LEFT JOIN tickets t ON t.user_id = u.id AND t.event_id = ue.event_id
		WHERE ue.event_id = $1 AND ue.is_banned = FALSE AND u.deleted_at IS NULL
	`
	args := []any{eventID}
	argIdx := 2

	if len(req.UserIDs) > 0 {
		baseQuery += fmt.Sprintf(" AND u.id = ANY($%d)", argIdx)
		args = append(args, req.UserIDs)
		argIdx++
	}

	if req.TeamID != nil && *req.TeamID != "" {
		baseQuery += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM team_members tm WHERE tm.user_id = u.id AND tm.team_id = $%d)", argIdx)
		args = append(args, *req.TeamID)
		argIdx++
	}

	if req.Role != nil && *req.Role != "" {
		baseQuery += fmt.Sprintf(" AND ue.role = $%d::event_role", argIdx)
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
		if err := rows.Scan(
			&u.UserID,
			&u.Name,
			&u.Email,
			&u.TicketID,
			&u.TicketCode,
			&u.IsCheckedIn,
			&u.EmailStatus,
			&u.EmailSentAt,
			&u.ReleasedAt,
		); err != nil {
			return nil, fmt.Errorf("scan eligible user: %w", err)
		}
		result = append(result, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate eligible users: %w", err)
	}

	return result, nil
}

// GetAttendanceAnalytics calculates aggregate attendance metrics, email status breakdown, hourly trends, and team progress for the active event.
func (r *Repository) GetAttendanceAnalytics(ctx context.Context) (*AnalyticsResponse, error) {
	eventID := resolveEventID(ctx)
	var totalIssued int
	var checkedInCount int
	var emailSentCount int
	var emailQueuedCount int
	var emailFailedCount int
	var emailSkippedDomainCount int

	countQuery := `
		SELECT 
			COUNT(*),
			COUNT(CASE WHEN is_checked_in THEN 1 END),
			COUNT(CASE WHEN email_status = 'sent' OR email_sent_at IS NOT NULL THEN 1 END),
			COUNT(CASE WHEN email_status = 'queued' THEN 1 END),
			COUNT(CASE WHEN email_status = 'failed' THEN 1 END),
			COUNT(CASE WHEN email_status = 'skipped_domain' THEN 1 END)
		FROM tickets
		WHERE event_id = $1;
	`
	err := r.db.QueryRow(ctx, countQuery, eventID).Scan(
		&totalIssued,
		&checkedInCount,
		&emailSentCount,
		&emailQueuedCount,
		&emailFailedCount,
		&emailSkippedDomainCount,
	)
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
		WHERE event_id = $1 AND is_checked_in = true AND checked_in_at IS NOT NULL
		GROUP BY hr
		ORDER BY hr ASC;
	`
	hourlyRows, err := r.db.Query(ctx, hourlyQuery, eventID)
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

	// Team completeness across competitions in this event
	teamQuery := `
		SELECT 
			tm.id,
			tm.name,
			COUNT(u.id) AS total_members,
			COUNT(CASE WHEN t.is_checked_in THEN 1 END) AS checked_in_members
		FROM teams tm
		JOIN competitions c ON c.id = tm.competition_id
		JOIN team_members tmm ON tmm.team_id = tm.id
		JOIN users u ON u.id = tmm.user_id AND u.deleted_at IS NULL
		LEFT JOIN tickets t ON t.user_id = u.id AND t.event_id = c.event_id
		WHERE c.event_id = $1
		GROUP BY tm.id, tm.name
		ORDER BY tm.name ASC;
	`
	teamRows, err := r.db.Query(ctx, teamQuery, eventID)
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
		TotalIssued:             totalIssued,
		CheckedInCount:          checkedInCount,
		AttendanceRate:          rate,
		EmailSentCount:          emailSentCount,
		EmailQueuedCount:        emailQueuedCount,
		EmailFailedCount:        emailFailedCount,
		EmailSkippedDomainCount: emailSkippedDomainCount,
		HourlyArrivals:          hourlyArrivals,
		TeamCompleteness:        teamSummaries,
	}, nil
}
