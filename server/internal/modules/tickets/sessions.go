package tickets

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"devclubxnst.com/cadence/pkg/audit"
	"devclubxnst.com/cadence/pkg/metrics"
	"devclubxnst.com/cadence/pkg/utils"
)

const (
	KindCheckIn    = "check_in"
	KindAttendance = "attendance"
	KindExit       = "exit"

	checkInCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	checkInCodeLength   = 6
)

var (
	ErrSessionNotFound                = errors.New("scan session not found")
	ErrSessionClosed                  = errors.New("scan session is closed")
	ErrNotCheckedIn                   = errors.New("attendee has not checked in at entry")
	ErrExitNotRequired                = errors.New("internal attendees do not exit-scan")
	ErrAlreadyScanned                 = errors.New("ticket already scanned in this session")
	ErrCompetitionRequiredForTeamScan = errors.New("competition_id is required when full-team-via-leader scanning is enabled")
	ErrInvalidCompetition             = errors.New("competition does not belong to this event")
	ErrInvalidCheckInCode             = errors.New("invalid or expired check-in code")
	ErrCheckInCodeConflict            = errors.New("check-in code is already in use by another scan session in this event")
	ErrNotEventMember                 = errors.New("you are not an active participant of this event")
)

// ---- DTOs -------------------------------------------------------------------------------------

type CreateSessionRequest struct {
	Name string `json:"name" validate:"required,min=2,max=150"`
	Kind string `json:"kind" validate:"omitempty,oneof=check_in attendance exit"`
	// MarkTeam makes a team leader's scan mark the whole team in the linked competition (false = per member).
	MarkTeam *bool `json:"mark_team"`
	// CompetitionID optionally links this scan session to a single competition in the event (required when MarkTeam is true).
	CompetitionID *string `json:"competition_id"`
	// GenerateCode automatically generates an initial check-in code for this session when true.
	GenerateCode bool `json:"generate_code"`
	// CheckInCode optionally sets a specific initial check-in code for this session.
	CheckInCode *string `json:"checkin_code"`
}

type GenerateCheckInCodeRequest struct {
	// Code optionally specifies a custom check-in code (2-32 chars). If omitted, a random 6-character code is generated.
	Code *string `json:"code"`
	// SessionID optionally targets a specific scan session on POST /tickets/checkin-code (defaults to the event's default Entry Check-In session).
	SessionID *string `json:"session_id"`
}

type CheckInByCodeRequest struct {
	Code string `json:"code" validate:"required"`
}

type SessionScanRequest struct {
	TicketCode string `json:"ticket_code" validate:"required"`
}

type SetSessionOpenRequest struct {
	IsOpen bool `json:"is_open"`
}

type ScanSession struct {
	ID                   string     `json:"id"`
	EventID              string     `json:"event_id"`
	CompetitionID        *string    `json:"competition_id,omitempty"`
	CompetitionSlug      *string    `json:"competition_slug,omitempty"`
	CompetitionName      *string    `json:"competition_name,omitempty"`
	Name                 string     `json:"name"`
	Kind                 string     `json:"kind"`
	IsDefault            bool       `json:"is_default"`
	IsOpen               bool       `json:"is_open"`
	MarkTeam             bool       `json:"mark_team"`
	CheckInCode          *string    `json:"checkin_code,omitempty"`
	CheckInCodeUpdatedAt *time.Time `json:"checkin_code_updated_at,omitempty"`
	ScannedCount         int        `json:"scanned_count"`
	CreatedByID          *string    `json:"created_by_id,omitempty"`
	CreatedByName        *string    `json:"created_by_name,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	ClosedAt             *time.Time `json:"closed_at,omitempty"`
}

type SessionScanResponse struct {
	Success        bool            `json:"success"`
	Message        string          `json:"message"`
	Session        ScanSession     `json:"session"`
	ScannedAt      string          `json:"scanned_at"`
	ScannedByID    string          `json:"scanned_by_id"`
	ScannedByName  string          `json:"scanned_by_name,omitempty"`
	ScannedByEmail string          `json:"scanned_by_email,omitempty"`
	Attendee       AttendeeDetails `json:"attendee"`
	Team           *TeamScanResult `json:"team,omitempty"`
}

type TeamMemberScanRecord struct {
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	ScannedAt   string `json:"scanned_at"`
	ScannedByID string `json:"scanned_by_id"`
}

// TeamScanResult reports what happened to the rest of the team when a leader was scanned.
type TeamScanResult struct {
	TeamID        string                 `json:"team_id"`
	TeamName      string                 `json:"team_name"`
	Marked        int                    `json:"marked"`
	Already       int                    `json:"already_scanned"`
	MarkedMembers []TeamMemberScanRecord `json:"marked_members,omitempty"`
	Skipped       []TeamScanSkip         `json:"skipped"`
}

type TeamScanSkip struct {
	Name   string `json:"name"`
	Reason string `json:"reason"` // not_checked_in | no_pass
}

// SessionScanRejection explains a refused scan. Code is stable for the scanner UI to branch on.
type SessionScanRejection struct {
	Error         string          `json:"error"`
	Code          string          `json:"code"`
	ScannedAt     string          `json:"scanned_at,omitempty"`
	ScannedByID   *string         `json:"scanned_by_id,omitempty"`
	ScannedByName *string         `json:"scanned_by_name,omitempty"`
	Attendee      AttendeeDetails `json:"attendee"`
}

type ReleaseTicketsResponse struct {
	TotalProcessed int `json:"total_processed"`
	Released       int `json:"released"`
	AlreadyHad     int `json:"already_had"`
	Skipped        int `json:"skipped"`
}

type SessionPerson struct {
	UserID        string  `json:"user_id"`
	Name          string  `json:"name"`
	Email         string  `json:"email"`
	Phone         *string `json:"phone_number,omitempty"`
	TeamID        *string `json:"team_id,omitempty"`
	TeamName      *string `json:"team_name,omitempty"`
	IsLeader      bool    `json:"is_leader"`
	External      bool    `json:"is_external"`
	ScannedAt     *string `json:"scanned_at,omitempty"`
	ScannedByID   *string `json:"scanned_by_id,omitempty"`
	ScannedByName *string `json:"scanned_by_name,omitempty"`
	// Present is true once the person has been scanned in this session.
	Present bool `json:"present"`
}

type SessionReport struct {
	Session  ScanSession     `json:"session"`
	Eligible int             `json:"eligible"`
	Scanned  int             `json:"scanned"`
	Missing  []SessionPerson `json:"missing"`
	Recent   []SessionPerson `json:"recent"`
	// People lists every expected attendee (present and absent), live from current teams and tickets.
	People []SessionPerson `json:"people"`
}

// ---- Repository -------------------------------------------------------------------------------

const scanSessionSelectQuery = `
	SELECT s.id, s.event_id, s.competition_id, c.slug, c.name,
	       s.name, s.kind, s.is_default, s.is_open, s.mark_team,
	       s.checkin_code, s.checkin_code_updated_at,
	       (SELECT COUNT(*) FROM scan_events e WHERE e.session_id = s.id),
	       s.created_by, u.name, s.created_at, s.closed_at
	FROM scan_sessions s
	LEFT JOIN competitions c ON c.id = s.competition_id
	LEFT JOIN users u ON u.id = s.created_by
`

func scanSessionRow(row pgx.Row, s *ScanSession) error {
	return row.Scan(
		&s.ID,
		&s.EventID,
		&s.CompetitionID,
		&s.CompetitionSlug,
		&s.CompetitionName,
		&s.Name,
		&s.Kind,
		&s.IsDefault,
		&s.IsOpen,
		&s.MarkTeam,
		&s.CheckInCode,
		&s.CheckInCodeUpdatedAt,
		&s.ScannedCount,
		&s.CreatedByID,
		&s.CreatedByName,
		&s.CreatedAt,
		&s.ClosedAt,
	)
}

func generateRandomCheckInCode() (string, error) {
	b := make([]byte, checkInCodeLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, checkInCodeLength)
	for i := range b {
		out[i] = checkInCodeAlphabet[int(b[i])%len(checkInCodeAlphabet)]
	}
	return string(out), nil
}

// GetSessionByCheckInCode looks up a scan session in the active event by its active checkin_code (case-insensitive).
func (r *Repository) GetSessionByCheckInCode(ctx context.Context, code string) (*ScanSession, error) {
	eventID := resolveEventID(ctx)
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if normalized == "" {
		return nil, ErrInvalidCheckInCode
	}
	var s ScanSession
	err := scanSessionRow(
		r.db.QueryRow(ctx, scanSessionSelectQuery+` WHERE s.event_id = $1 AND s.checkin_code IS NOT NULL AND UPPER(s.checkin_code) = $2 LIMIT 1`, eventID, normalized),
		&s,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCheckInCode
		}
		return nil, fmt.Errorf("get session by check-in code: %w", err)
	}
	return &s, nil
}

// SetSessionCheckInCode sets or rotates (or clears when code == nil) the check-in code for a scan session in the active event.
func (r *Repository) SetSessionCheckInCode(ctx context.Context, sessionID string, code *string) (*ScanSession, error) {
	eventID := resolveEventID(ctx)
	var cleanCode *string
	if code != nil {
		c := strings.ToUpper(strings.TrimSpace(*code))
		if c != "" {
			cleanCode = &c
		}
	}

	var updatedAt *time.Time
	if cleanCode != nil {
		now := time.Now()
		updatedAt = &now
	}

	tag, err := r.db.Exec(ctx, `
		UPDATE scan_sessions
		SET checkin_code = $1,
		    checkin_code_updated_at = $2
		WHERE id = $3 AND event_id = $4
	`, cleanCode, updatedAt, sessionID, eventID)
	if err != nil {
		if strings.Contains(err.Error(), "idx_scan_sessions_event_checkin_code") || strings.Contains(err.Error(), "duplicate key") {
			return nil, ErrCheckInCodeConflict
		}
		return nil, fmt.Errorf("set session check-in code: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrSessionNotFound
	}
	return r.GetSession(ctx, sessionID)
}

// EnsureDefaultCheckInSession guarantees that the active event has its default Entry Check-In scan session.
func (r *Repository) EnsureDefaultCheckInSession(ctx context.Context) (*ScanSession, error) {
	eventID := resolveEventID(ctx)
	_, err := r.db.Exec(ctx, `
		INSERT INTO scan_sessions (event_id, name, kind, is_default, is_open, mark_team)
		VALUES ($1, 'Entry Check-In', 'check_in', TRUE, TRUE, FALSE)
		ON CONFLICT (event_id) WHERE is_default = TRUE DO NOTHING
	`, eventID)
	if err != nil {
		return nil, fmt.Errorf("ensure default check-in session: %w", err)
	}

	var s ScanSession
	err = scanSessionRow(
		r.db.QueryRow(ctx, scanSessionSelectQuery+` WHERE s.event_id = $1 AND s.is_default = TRUE LIMIT 1`, eventID),
		&s,
	)
	if err != nil {
		return nil, fmt.Errorf("load default check-in session: %w", err)
	}
	return &s, nil
}

// RecordDefaultCheckInScan records a scan in the active event's default Check-In session when a user checks in via /tickets/check-in.
func (r *Repository) RecordDefaultCheckInScan(ctx context.Context, ticketID, scannerID string) error {
	defSess, err := r.EnsureDefaultCheckInSession(ctx)
	if err != nil {
		return err
	}
	_, _, err = r.RecordScan(ctx, defSess.ID, ticketID, scannerID)
	return err
}

// CompetitionBelongsToEvent verifies that competitionID belongs to the active event.
func (r *Repository) CompetitionBelongsToEvent(ctx context.Context, competitionID string) (bool, error) {
	eventID := resolveEventID(ctx)
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM competitions WHERE id = $1 AND event_id = $2)`, competitionID, eventID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check competition event: %w", err)
	}
	return exists, nil
}

func (r *Repository) CreateSession(ctx context.Context, name, kind string, markTeam bool, competitionID *string, createdBy string) (*ScanSession, error) {
	eventID := resolveEventID(ctx)
	var compID string
	if competitionID != nil {
		compID = strings.TrimSpace(*competitionID)
	}
	var sessionID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO scan_sessions (event_id, competition_id, name, kind, is_default, mark_team, created_by)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, FALSE, $5, NULLIF($6, '')::uuid)
		RETURNING id`,
		eventID, compID, name, kind, markTeam, createdBy,
	).Scan(&sessionID)
	if err != nil {
		return nil, fmt.Errorf("create scan session: %w", err)
	}
	return r.GetSession(ctx, sessionID)
}

func (r *Repository) ListSessions(ctx context.Context) ([]ScanSession, error) {
	_, _ = r.EnsureDefaultCheckInSession(ctx)
	eventID := resolveEventID(ctx)
	rows, err := r.db.Query(ctx, scanSessionSelectQuery+`
		WHERE s.event_id = $1
		ORDER BY s.is_default DESC, s.is_open DESC, s.created_at DESC`, eventID)
	if err != nil {
		return nil, fmt.Errorf("list scan sessions: %w", err)
	}
	defer rows.Close()
	out := make([]ScanSession, 0)
	for rows.Next() {
		var s ScanSession
		if err := scanSessionRow(rows, &s); err != nil {
			return nil, fmt.Errorf("scan session row: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) GetSession(ctx context.Context, id string) (*ScanSession, error) {
	eventID := resolveEventID(ctx)
	var s ScanSession
	err := scanSessionRow(
		r.db.QueryRow(ctx, scanSessionSelectQuery+` WHERE s.id = $1 AND s.event_id = $2`, id, eventID),
		&s,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("get scan session: %w", err)
	}
	return &s, nil
}

func (r *Repository) SetSessionOpen(ctx context.Context, id string, open bool) (*ScanSession, error) {
	eventID := resolveEventID(ctx)
	tag, err := r.db.Exec(ctx, `
		UPDATE scan_sessions
		SET is_open = $2, closed_at = CASE WHEN $2 THEN NULL ELSE CURRENT_TIMESTAMP END
		WHERE id = $1 AND event_id = $3`, id, open, eventID)
	if err != nil {
		return nil, fmt.Errorf("toggle scan session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrSessionNotFound
	}
	return r.GetSession(ctx, id)
}

// RecordScan inserts the scan with who scanned whom and when; returns false when this ticket was already scanned in the session.
func (r *Repository) RecordScan(ctx context.Context, sessionID, ticketID, scannerID string) (time.Time, bool, error) {
	var at time.Time
	err := r.db.QueryRow(ctx, `
		INSERT INTO scan_events (session_id, ticket_id, scanned_by) VALUES ($1, $2, NULLIF($3, '')::uuid)
		ON CONFLICT (session_id, ticket_id) DO NOTHING
		RETURNING scanned_at`, sessionID, ticketID, scannerID).Scan(&at)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("record scan: %w", err)
	}
	return at, true, nil
}

func (r *Repository) GetScan(ctx context.Context, sessionID, ticketID string) (time.Time, *string, *string, error) {
	var at time.Time
	var byID, byName *string
	err := r.db.QueryRow(ctx, `
		SELECT e.scanned_at, e.scanned_by::text, u.name
		FROM scan_events e
		LEFT JOIN users u ON u.id = e.scanned_by
		WHERE e.session_id = $1 AND e.ticket_id = $2`, sessionID, ticketID).Scan(&at, &byID, &byName)
	if err != nil {
		return time.Time{}, nil, nil, fmt.Errorf("get scan: %w", err)
	}
	return at, byID, byName, nil
}

// ReleaseTicket marks a ticket visible on the participant dashboard without sending email.
func (r *Repository) ReleaseTicket(ctx context.Context, ticketID string) error {
	_, err := r.db.Exec(ctx, `UPDATE tickets SET released_at = COALESCE(released_at, CURRENT_TIMESTAMP) WHERE id = $1`, ticketID)
	if err != nil {
		return fmt.Errorf("release ticket: %w", err)
	}
	return nil
}

func (r *Repository) CountScans(ctx context.Context, sessionID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM scan_events WHERE session_id = $1`, sessionID).Scan(&n)
	return n, err
}

// GetTicketByCodeForCompetition looks up a ticket in the active event and resolves the attendee's team
// in competitionID (if non-nil) or in the event otherwise.
func (r *Repository) GetTicketByCodeForCompetition(ctx context.Context, ticketCode string, competitionID *string) (*TicketAttendeeInfo, error) {
	eventID := resolveEventID(ctx)
	var compID string
	if competitionID != nil {
		compID = *competitionID
	}
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
			WHERE tm.user_id = u.id
			  AND c.event_id = t.event_id
			  AND ($3 = '' OR tm.competition_id = $3::uuid)
			ORDER BY tm.joined_at DESC
			LIMIT 1
		) tm_sub ON TRUE
		LEFT JOIN users v ON v.id = t.checked_in_by
		WHERE t.event_id = $1 AND t.ticket_code = $2;
	`
	var info TicketAttendeeInfo
	err := r.db.QueryRow(ctx, query, eventID, ticketCode, compID).Scan(
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
		return nil, fmt.Errorf("query ticket by code for competition: %w", err)
	}
	return &info, nil
}

// MarkTeamOfLeader marks every member of the team led by leaderID in the session's linked competition.
// Returns nil when the scanned user does not lead a team in that competition.
// Each team member scan is recorded in scan_events with scanned_by = scannerID and scanned_at = NOW().
func (r *Repository) MarkTeamOfLeader(
	ctx context.Context,
	sessionID string,
	competitionID *string,
	leaderID, scannerID string,
	mintCodeFn func() (string, error),
) (*TeamScanResult, error) {
	eventID := resolveEventID(ctx)
	var compID string
	if competitionID != nil {
		compID = *competitionID
	}

	var teamID, teamName string
	err := r.db.QueryRow(ctx, `
		SELECT t.id, t.name
		FROM teams t
		JOIN competitions c ON c.id = t.competition_id
		WHERE t.leader_id = $1
		  AND c.event_id = $2
		  AND ($3 = '' OR t.competition_id = $3::uuid)
		ORDER BY t.created_at DESC
		LIMIT 1
	`, leaderID, eventID, compID).Scan(&teamID, &teamName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find led team: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.name, u.email, t.id
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		LEFT JOIN tickets t ON t.user_id = u.id AND t.event_id = $3
		WHERE tm.team_id = $1 AND u.id <> $2 AND u.deleted_at IS NULL
		ORDER BY u.name`, teamID, leaderID, eventID)
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	type member struct {
		userID   string
		name     string
		email    string
		ticketID *string
	}
	var members []member
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.userID, &m.name, &m.email, &m.ticketID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		members = append(members, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	res := &TeamScanResult{
		TeamID:        teamID,
		TeamName:      teamName,
		MarkedMembers: []TeamMemberScanRecord{},
		Skipped:       []TeamScanSkip{},
	}
	for _, m := range members {
		tktID := m.ticketID
		// If a team member hasn't opened their dashboard yet to mint their ticket, mint it automatically so the leader's scan marks them!
		if tktID == nil && mintCodeFn != nil {
			if code, codeErr := mintCodeFn(); codeErr == nil {
				if minted, mintErr := r.GetOrCreateReleasedTicket(ctx, m.userID, code); mintErr == nil {
					tktID = &minted.ID
				}
			}
		}

		switch {
		case tktID == nil:
			res.Skipped = append(res.Skipped, TeamScanSkip{Name: m.name, Reason: "no_pass"})
		default:
			at, recorded, err := r.RecordScan(ctx, sessionID, *tktID, scannerID)
			if err != nil {
				return nil, err
			}
			if recorded {
				res.Marked++
				res.MarkedMembers = append(res.MarkedMembers, TeamMemberScanRecord{
					UserID:      m.userID,
					Name:        m.name,
					Email:       m.email,
					ScannedAt:   at.Format(time.RFC3339),
					ScannedByID: scannerID,
				})
			} else {
				res.Already++
			}
		}
	}
	return res, nil
}

// sessionPeople returns every expected participant in the active event (or linked competition)
// with their scan timestamp and the volunteer/admin who scanned them in the session (if any).
func (r *Repository) sessionPeople(ctx context.Context, sess *ScanSession) ([]SessionPerson, error) {
	eventID := resolveEventID(ctx)
	var compID string
	if sess.CompetitionID != nil {
		compID = *sess.CompetitionID
	}

	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.name, u.email, u.phone_number,
		       tm_sub.team_id, tm_sub.team_name, COALESCE(tm_sub.is_leader, FALSE),
		       e.scanned_at, e.scanned_by::text, scanner.name
		FROM user_events ue
		JOIN users u ON u.id = ue.user_id AND u.deleted_at IS NULL
		LEFT JOIN tickets t ON t.user_id = u.id AND t.event_id = ue.event_id
		LEFT JOIN LATERAL (
			SELECT tm.team_id::text AS team_id, tm_t.name AS team_name, (tm_t.leader_id = u.id) AS is_leader
			FROM team_members tm
			JOIN teams tm_t ON tm_t.id = tm.team_id
			JOIN competitions c ON c.id = tm_t.competition_id
			WHERE tm.user_id = u.id
			  AND c.event_id = ue.event_id
			  AND ($4 = '' OR tm.competition_id = $4::uuid)
			ORDER BY tm.joined_at DESC
			LIMIT 1
		) tm_sub ON TRUE
		LEFT JOIN scan_events e ON e.ticket_id = t.id AND e.session_id = $1
		LEFT JOIN users scanner ON scanner.id = e.scanned_by
		WHERE ue.event_id = $3
		  AND ue.role = 'participant'
		  AND ue.is_banned = FALSE
		  AND ($2 <> 'exit' OR COALESCE(t.is_checked_in, FALSE) = TRUE)
		  AND ($4 = '' OR tm_sub.team_id IS NOT NULL OR e.scanned_at IS NOT NULL)
		ORDER BY u.name`,
		sess.ID, sess.Kind, eventID, compID,
	)
	if err != nil {
		return nil, fmt.Errorf("query session people: %w", err)
	}
	defer rows.Close()
	var out []SessionPerson
	for rows.Next() {
		var p SessionPerson
		var at *time.Time
		if err := rows.Scan(
			&p.UserID,
			&p.Name,
			&p.Email,
			&p.Phone,
			&p.TeamID,
			&p.TeamName,
			&p.IsLeader,
			&at,
			&p.ScannedByID,
			&p.ScannedByName,
		); err != nil {
			return nil, fmt.Errorf("scan session person: %w", err)
		}
		if at != nil {
			s := at.Format(time.RFC3339)
			p.ScannedAt = &s
			p.Present = true
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---- Service ----------------------------------------------------------------------------------

func (s *Service) isExternal(ctx context.Context, email string) (bool, error) {
	domains, err := s.InternalDomains(ctx)
	if err != nil {
		return false, err
	}
	return !utils.IsEmailExcluded(email, domains), nil
}

func (s *Service) CreateSession(ctx context.Context, req CreateSessionRequest, actorID, actorEmail, actorRole, ip, ua string) (*ScanSession, error) {
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = KindAttendance
	}

	var compID *string
	if req.CompetitionID != nil && strings.TrimSpace(*req.CompetitionID) != "" {
		trimmed := strings.TrimSpace(*req.CompetitionID)
		belongs, err := s.repo.CompetitionBelongsToEvent(ctx, trimmed)
		if err != nil {
			return nil, err
		}
		if !belongs {
			return nil, ErrInvalidCompetition
		}
		compID = &trimmed
	}

	markTeam := false
	if req.MarkTeam != nil {
		markTeam = *req.MarkTeam && kind == KindAttendance
	} else if compID != nil && kind == KindAttendance {
		markTeam = true
	}

	if markTeam && compID == nil {
		return nil, ErrCompetitionRequiredForTeamScan
	}

	sess, err := s.repo.CreateSession(ctx, req.Name, kind, markTeam, compID, actorID)
	if err != nil {
		return nil, err
	}

	if req.CheckInCode != nil && strings.TrimSpace(*req.CheckInCode) != "" {
		sess, err = s.GenerateSessionCheckInCode(ctx, sess.ID, req.CheckInCode, actorID, actorEmail, actorRole, ip, ua)
		if err != nil {
			return nil, err
		}
	} else if req.GenerateCode {
		sess, err = s.GenerateSessionCheckInCode(ctx, sess.ID, nil, actorID, actorEmail, actorRole, ip, ua)
		if err != nil {
			return nil, err
		}
	}

	s.recordAudit(ctx, audit.ActionScanSessionCreate, audit.TargetScanSession, &sess.ID,
		fmt.Sprintf("%s created %s scan session %q (mark_team=%t)", actorEmail, kind, req.Name, markTeam),
		&actorID, &actorEmail, &actorRole,
		map[string]any{"name": req.Name, "kind": kind, "mark_team": markTeam, "competition_id": compID, "checkin_code": sess.CheckInCode}, &ip, &ua)
	return sess, nil
}

// GenerateSessionCheckInCode generates or rotates a check-in code for a specific scan session (or the default Check-In session if sessionID == "").
// Generating a new code immediately replaces and invalidates the previous code on that session.
func (s *Service) GenerateSessionCheckInCode(
	ctx context.Context,
	sessionID string,
	customCode *string,
	actorID, actorEmail, actorRole, ip, ua string,
) (*ScanSession, error) {
	if strings.TrimSpace(sessionID) == "" {
		defSess, err := s.repo.EnsureDefaultCheckInSession(ctx)
		if err != nil {
			return nil, err
		}
		sessionID = defSess.ID
	}

	var sess *ScanSession
	if customCode != nil && strings.TrimSpace(*customCode) != "" {
		code := strings.ToUpper(strings.TrimSpace(*customCode))
		if len(code) < 2 || len(code) > 32 {
			return nil, ErrInvalidCheckInCode
		}
		var err error
		sess, err = s.repo.SetSessionCheckInCode(ctx, sessionID, &code)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		for attempt := 0; attempt < 5; attempt++ {
			code, genErr := generateRandomCheckInCode()
			if genErr != nil {
				return nil, genErr
			}
			sess, err = s.repo.SetSessionCheckInCode(ctx, sessionID, &code)
			if errors.Is(err, ErrCheckInCodeConflict) {
				continue
			}
			break
		}
		if err != nil {
			return nil, err
		}
	}

	s.recordAudit(ctx, audit.ActionScanSessionCodeGen, audit.TargetScanSession, &sess.ID,
		fmt.Sprintf("%s generated new check-in code for session %q", actorEmail, sess.Name),
		&actorID, &actorEmail, &actorRole,
		map[string]any{"session_id": sess.ID, "session_name": sess.Name, "checkin_code": sess.CheckInCode}, &ip, &ua)

	return sess, nil
}

// ClearSessionCheckInCode disables code-based self-check-in for the given scan session (or default Check-In session if sessionID == "").
func (s *Service) ClearSessionCheckInCode(
	ctx context.Context,
	sessionID string,
	actorID, actorEmail, actorRole, ip, ua string,
) (*ScanSession, error) {
	if strings.TrimSpace(sessionID) == "" {
		defSess, err := s.repo.EnsureDefaultCheckInSession(ctx)
		if err != nil {
			return nil, err
		}
		sessionID = defSess.ID
	}

	sess, err := s.repo.SetSessionCheckInCode(ctx, sessionID, nil)
	if err != nil {
		return nil, err
	}

	s.recordAudit(ctx, audit.ActionScanSessionCodeClear, audit.TargetScanSession, &sess.ID,
		fmt.Sprintf("%s cleared check-in code for session %q", actorEmail, sess.Name),
		&actorID, &actorEmail, &actorRole,
		map[string]any{"session_id": sess.ID, "session_name": sess.Name, "checkin_code": nil}, &ip, &ua)

	return sess, nil
}

// CheckInByCode allows an authenticated participant to check themselves in directly using an active check-in code
// generated by an event admin or owner.
func (s *Service) CheckInByCode(
	ctx context.Context,
	rawCode string,
	userID, userEmail, userRole, ip, ua string,
) (*SessionScanResponse, *SessionScanRejection, error) {
	isMember, err := s.repo.IsEventMember(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if !isMember {
		return nil, nil, ErrNotEventMember
	}

	sess, err := s.repo.GetSessionByCheckInCode(ctx, rawCode)
	if err != nil {
		return nil, nil, err
	}

	// Ensure the participant has a ticket row minted in this event so the scan can be recorded against their ticket
	ticket, err := s.repo.GetTicketByUserID(ctx, userID)
	if err != nil && !errors.Is(err, ErrTicketNotFound) {
		return nil, nil, err
	}
	if ticket == nil {
		code, codeErr := s.generateTicketCode()
		if codeErr != nil {
			return nil, nil, fmt.Errorf("generate ticket code: %w", codeErr)
		}
		ticket, err = s.repo.GetOrCreateReleasedTicket(ctx, userID, code)
		if err != nil {
			return nil, nil, err
		}
	}

	// Delegate to ScanInSession using the participant's own ticket code and userID as the self-check-in actor
	resp, rejection, err := s.ScanInSession(ctx, sess.ID, ticket.TicketCode, userID, userEmail, userRole, ip, ua)
	if err == nil && resp != nil {
		s.recordAudit(ctx, audit.ActionTicketCodeCheckIn, audit.TargetScanSession, &sess.ID,
			fmt.Sprintf("Participant %s self-checked in to %q via code", userEmail, sess.Name),
			&userID, &userEmail, &userRole,
			map[string]any{"session_id": sess.ID, "session_name": sess.Name, "kind": sess.Kind, "ticket_id": ticket.ID},
			&ip, &ua)
	}
	return resp, rejection, err
}

func (s *Service) ListSessions(ctx context.Context) ([]ScanSession, error) {
	return s.repo.ListSessions(ctx)
}

func (s *Service) SetSessionOpen(ctx context.Context, id string, open bool, actorID, actorEmail, actorRole, ip, ua string) (*ScanSession, error) {
	sess, err := s.repo.SetSessionOpen(ctx, id, open)
	if err != nil {
		return nil, err
	}
	state := "closed"
	if open {
		state = "opened"
	}
	s.recordAudit(ctx, audit.ActionScanSessionToggle, audit.TargetScanSession, &sess.ID,
		fmt.Sprintf("%s %s scan session %q", actorEmail, state, sess.Name),
		&actorID, &actorEmail, &actorRole, map[string]any{"is_open": open}, &ip, &ua)
	return sess, nil
}

// ScanInSession validates and records a check-in, attendance, or exit scan (recording who scanned whom and when,
// and supporting either per-member scanning or full-team-via-leader scanning in the linked competition).
func (s *Service) ScanInSession(
	ctx context.Context, sessionID, ticketCode string,
	scannerID, scannerEmail, scannerRole, ip, ua string,
) (*SessionScanResponse, *SessionScanRejection, error) {
	sess, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	// Refusals are audited so a gate dispute can be traced. The ticket code itself is never logged: it is the credential.
	auditRejected := func(code, desc string, targetID *string, extra map[string]any) {
		meta := map[string]any{"session_id": sess.ID, "session_name": sess.Name, "kind": sess.Kind, "reason": code}
		for k, v := range extra {
			meta[k] = v
		}
		s.recordAudit(ctx, audit.ActionTicketScanRejected, audit.TargetScanSession, targetID,
			desc, &scannerID, &scannerEmail, &scannerRole, meta, &ip, &ua)
	}

	if !sess.IsOpen {
		auditRejected("session_closed", fmt.Sprintf("%s scanned into closed %s session %q", scannerEmail, sess.Kind, sess.Name), &sess.ID, nil)
		return nil, &SessionScanRejection{Error: "This scan session is closed", Code: "session_closed"}, ErrSessionClosed
	}

	info, err := s.repo.GetTicketByCodeForCompetition(ctx, ticketCode, sess.CompetitionID)
	if err != nil {
		metrics.InstrumentTicketScan("invalid")
		if errors.Is(err, ErrTicketNotFound) {
			auditRejected("invalid_ticket", fmt.Sprintf("%s scanned an unknown ticket in %s session %q", scannerEmail, sess.Kind, sess.Name), &sess.ID, nil)
		}
		return nil, nil, err
	}
	attendee := AttendeeDetails{
		ID: info.UserID, Name: info.UserName, Email: info.UserEmail, Role: info.UserRole,
		PhoneNumber: info.UserPhone, TeamID: info.TeamID, TeamName: info.TeamName,
	}
	reject := func(code, msg string, sentinel error) (*SessionScanResponse, *SessionScanRejection, error) {
		metrics.InstrumentTicketScan(code)
		auditRejected(code, fmt.Sprintf("%s scan refused for %s in %s session %q: %s", scannerEmail, info.UserEmail, sess.Kind, sess.Name, msg),
			&info.TicketID, map[string]any{"attendee_id": info.UserID, "attendee_email": info.UserEmail})
		return nil, &SessionScanRejection{Error: msg, Code: code, Attendee: attendee}, sentinel
	}

	// Entry check-in gates only the exit gate; session attendance is open to every ticket holder.
	if sess.Kind == KindExit && !info.IsCheckedIn {
		return reject("not_checked_in", "This attendee has not checked in at entry", ErrNotCheckedIn)
	}
	if sess.Kind == KindExit {
		external, err := s.isExternal(ctx, info.UserEmail)
		if err != nil {
			return nil, nil, err
		}
		if !external {
			return reject("exit_not_required", "Internal attendees do not exit-scan", ErrExitNotRequired)
		}
	}

	at, recorded, err := s.repo.RecordScan(ctx, sess.ID, info.TicketID, scannerID)
	if err != nil {
		return nil, nil, err
	}
	if !recorded {
		prevAt, prevByID, prevByName, err := s.repo.GetScan(ctx, sess.ID, info.TicketID)
		if err != nil {
			return nil, nil, err
		}
		metrics.InstrumentTicketScan("duplicate")
		s.recordAudit(ctx, audit.ActionTicketDuplicateScan, audit.TargetTicket, &info.TicketID,
			fmt.Sprintf("Duplicate %s scan of %s in %q by %s", sess.Kind, info.UserEmail, sess.Name, scannerEmail),
			&scannerID, &scannerEmail, &scannerRole,
			map[string]any{
				"session_id": sess.ID, "session_name": sess.Name, "kind": sess.Kind,
				"attendee_id": info.UserID, "attendee_email": info.UserEmail,
				"prior_scanned_at": prevAt.Format(time.RFC3339),
			}, &ip, &ua)
		return nil, &SessionScanRejection{
			Error: "Already scanned in this session", Code: "duplicate",
			ScannedAt: prevAt.Format(time.RFC3339), ScannedByID: prevByID, ScannedByName: prevByName, Attendee: attendee,
		}, ErrAlreadyScanned
	}

	// If this is the default Check-In session (or a check_in session), synchronize tickets.is_checked_in as well.
	if sess.IsDefault || sess.Kind == KindCheckIn {
		_, _, _ = s.repo.MarkCheckedIn(ctx, info.TicketID, scannerID)
	}

	scannerName, _, _ := s.repo.GetUserByID(ctx, scannerID)

	metrics.InstrumentTicketScan("valid")
	s.recordAudit(ctx, audit.ActionTicketSessionScan, audit.TargetTicket, &info.TicketID,
		fmt.Sprintf("%s scanned %s in %s %q", scannerEmail, info.UserEmail, sess.Kind, sess.Name),
		&scannerID, &scannerEmail, &scannerRole,
		map[string]any{
			"session_id":     sess.ID,
			"session_name":   sess.Name,
			"kind":           sess.Kind,
			"attendee_id":    info.UserID,
			"attendee_email": info.UserEmail,
			"scanned_by_id":  scannerID,
		},
		&ip, &ua)

	resp := &SessionScanResponse{
		Success:        true,
		Message:        "Scan recorded",
		Session:        *sess,
		ScannedAt:      at.Format(time.RFC3339),
		ScannedByID:    scannerID,
		ScannedByName:  scannerName,
		ScannedByEmail: scannerEmail,
		Attendee:       attendee,
	}

	if sess.Kind == KindAttendance && sess.MarkTeam {
		team, err := s.repo.MarkTeamOfLeader(ctx, sess.ID, sess.CompetitionID, info.UserID, scannerID, s.generateTicketCode)
		if err != nil {
			// The leader is already recorded; surface the failure rather than hide a partial team.
			return nil, nil, fmt.Errorf("mark team: %w", err)
		}
		if team != nil {
			resp.Team = team
			resp.Message = fmt.Sprintf("Team %s: %d marked present", team.TeamName, team.Marked+1)
			s.recordAudit(ctx, audit.ActionTicketSessionScan, audit.TargetScanSession, &sess.ID,
				fmt.Sprintf("%s scanned team leader %s; team %q marked in %q", scannerEmail, info.UserEmail, team.TeamName, sess.Name),
				&scannerID, &scannerEmail, &scannerRole,
				map[string]any{"team_id": team.TeamID, "team": team.TeamName, "marked": team.Marked, "already": team.Already, "skipped": len(team.Skipped)},
				&ip, &ua)
		}
	}

	resp.Session.ScannedCount, _ = s.repo.CountScans(ctx, sess.ID)
	return resp, nil, nil
}

func (s *Service) GetSessionReport(ctx context.Context, sessionID string) (*SessionReport, error) {
	sess, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	people, err := s.repo.sessionPeople(ctx, sess)
	if err != nil {
		return nil, err
	}

	domains, err := s.InternalDomains(ctx)
	if err != nil {
		return nil, err
	}
	report := &SessionReport{Session: *sess, Missing: []SessionPerson{}, Recent: []SessionPerson{}, People: []SessionPerson{}}
	for _, p := range people {
		p.External = !utils.IsEmailExcluded(p.Email, domains)
		if sess.Kind == KindExit && !p.External {
			continue // internal attendees are not expected to exit-scan
		}
		report.Eligible++
		report.People = append(report.People, p)
		if p.ScannedAt != nil {
			report.Scanned++
			report.Recent = append(report.Recent, p)
		} else {
			report.Missing = append(report.Missing, p)
		}
	}
	// Most recent scans first, capped at 20.
	sortRecent(report.Recent)
	if len(report.Recent) > 20 {
		report.Recent = report.Recent[:20]
	}
	return report, nil
}

func sortRecent(p []SessionPerson) {
	for i := 1; i < len(p); i++ {
		for j := i; j > 0 && *p[j].ScannedAt > *p[j-1].ScannedAt; j-- {
			p[j], p[j-1] = p[j-1], p[j]
		}
	}
}

// ReleaseTickets marks event tickets as released globally and creates dashboard tickets for all current event members, sending no email.
func (s *Service) ReleaseTickets(ctx context.Context, req BulkSendTicketsRequest, actorID, actorEmail, actorRole, ip, ua string) (*ReleaseTicketsResponse, error) {
	if err := s.setTicketsReleased(ctx, true, actorID); err != nil {
		return nil, err
	}

	eligible, err := s.repo.FindEligibleUsers(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &ReleaseTicketsResponse{TotalProcessed: 0}
	for _, u := range eligible {
		resp.TotalProcessed++

		if u.TicketCode != "" {
			if u.ReleasedAt != nil {
				resp.AlreadyHad++
			} else {
				if err := s.repo.ReleaseTicket(ctx, u.TicketID); err != nil {
					resp.Skipped++
					continue
				}
				resp.Released++
			}
			continue
		}

		code, err := s.generateTicketCode()
		if err != nil {
			resp.Skipped++
			continue
		}
		if _, err := s.repo.GetOrCreateReleasedTicket(ctx, u.UserID, code); err != nil {
			resp.Skipped++
			continue
		}
		resp.Released++
	}
	s.recordAudit(ctx, audit.ActionTicketReleased, audit.TargetTicket, nil,
		fmt.Sprintf("%s released %d passes to the dashboard without email", actorEmail, resp.Released),
		&actorID, &actorEmail, &actorRole,
		map[string]any{"released": resp.Released, "already_had": resp.AlreadyHad, "skipped": resp.Skipped, "tickets_released": true},
		&ip, &ua)
	return resp, nil
}
