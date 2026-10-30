package tickets

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"devclubxnst.com/cadence/pkg/audit"
	"devclubxnst.com/cadence/pkg/metrics"
	"devclubxnst.com/cadence/pkg/utils"
)

const (
	KindAttendance = "attendance"
	KindExit       = "exit"
)

var (
	ErrSessionNotFound = errors.New("scan session not found")
	ErrSessionClosed   = errors.New("scan session is closed")
	ErrNotCheckedIn    = errors.New("attendee has not checked in at entry")
	ErrExitNotRequired = errors.New("internal attendees do not exit-scan")
	ErrAlreadyScanned  = errors.New("ticket already scanned in this session")
)

// ---- DTOs -------------------------------------------------------------------------------------

type CreateSessionRequest struct {
	Name string `json:"name" validate:"required,min=2,max=150"`
	Kind string `json:"kind" validate:"required,oneof=attendance exit"`
	// MarkTeam makes a team leader's scan mark the whole team. Defaults to true for attendance rounds.
	MarkTeam *bool `json:"mark_team"`
}

type SessionScanRequest struct {
	TicketCode string `json:"ticket_code" validate:"required"`
}

type SetSessionOpenRequest struct {
	IsOpen bool `json:"is_open"`
}

type ScanSession struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Kind         string     `json:"kind"`
	IsOpen       bool       `json:"is_open"`
	MarkTeam     bool       `json:"mark_team"`
	ScannedCount int        `json:"scanned_count"`
	CreatedAt    time.Time  `json:"created_at"`
	ClosedAt     *time.Time `json:"closed_at,omitempty"`
}

type SessionScanResponse struct {
	Success   bool            `json:"success"`
	Message   string          `json:"message"`
	Session   ScanSession     `json:"session"`
	ScannedAt string          `json:"scanned_at"`
	Attendee  AttendeeDetails `json:"attendee"`
	Team      *TeamScanResult `json:"team,omitempty"`
}

// TeamScanResult reports what happened to the rest of the team when a leader was scanned.
type TeamScanResult struct {
	TeamName string         `json:"team_name"`
	Marked   int            `json:"marked"`
	Already  int            `json:"already_scanned"`
	Skipped  []TeamScanSkip `json:"skipped"`
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
	UserID    string  `json:"user_id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	Phone     *string `json:"phone_number,omitempty"`
	TeamName  *string `json:"team_name,omitempty"`
	External  bool    `json:"is_external"`
	ScannedAt *string `json:"scanned_at,omitempty"`
}

type SessionReport struct {
	Session  ScanSession     `json:"session"`
	Eligible int             `json:"eligible"`
	Scanned  int             `json:"scanned"`
	Missing  []SessionPerson `json:"missing"`
	Recent   []SessionPerson `json:"recent"`
}

// ---- Repository -------------------------------------------------------------------------------

func (r *Repository) CreateSession(ctx context.Context, name, kind string, markTeam bool, createdBy string) (*ScanSession, error) {
	var s ScanSession
	err := r.db.QueryRow(ctx, `
		INSERT INTO scan_sessions (name, kind, mark_team, created_by) VALUES ($1, $2, $3, $4)
		RETURNING id, name, kind, is_open, mark_team, created_at, closed_at`, name, kind, markTeam, createdBy,
	).Scan(&s.ID, &s.Name, &s.Kind, &s.IsOpen, &s.MarkTeam, &s.CreatedAt, &s.ClosedAt)
	if err != nil {
		return nil, fmt.Errorf("create scan session: %w", err)
	}
	return &s, nil
}

func (r *Repository) ListSessions(ctx context.Context) ([]ScanSession, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.id, s.name, s.kind, s.is_open, s.mark_team, s.created_at, s.closed_at,
		       (SELECT COUNT(*) FROM scan_events e WHERE e.session_id = s.id)
		FROM scan_sessions s ORDER BY s.is_open DESC, s.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list scan sessions: %w", err)
	}
	defer rows.Close()
	out := make([]ScanSession, 0)
	for rows.Next() {
		var s ScanSession
		if err := rows.Scan(&s.ID, &s.Name, &s.Kind, &s.IsOpen, &s.MarkTeam, &s.CreatedAt, &s.ClosedAt, &s.ScannedCount); err != nil {
			return nil, fmt.Errorf("scan session row: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) GetSession(ctx context.Context, id string) (*ScanSession, error) {
	var s ScanSession
	err := r.db.QueryRow(ctx, `
		SELECT s.id, s.name, s.kind, s.is_open, s.mark_team, s.created_at, s.closed_at,
		       (SELECT COUNT(*) FROM scan_events e WHERE e.session_id = s.id)
		FROM scan_sessions s WHERE s.id = $1`, id,
	).Scan(&s.ID, &s.Name, &s.Kind, &s.IsOpen, &s.MarkTeam, &s.CreatedAt, &s.ClosedAt, &s.ScannedCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("get scan session: %w", err)
	}
	return &s, nil
}

func (r *Repository) SetSessionOpen(ctx context.Context, id string, open bool) (*ScanSession, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE scan_sessions
		SET is_open = $2, closed_at = CASE WHEN $2 THEN NULL ELSE CURRENT_TIMESTAMP END
		WHERE id = $1`, id, open)
	if err != nil {
		return nil, fmt.Errorf("toggle scan session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrSessionNotFound
	}
	return r.GetSession(ctx, id)
}

// RecordScan inserts the scan; returns false when this ticket was already scanned in the session.
func (r *Repository) RecordScan(ctx context.Context, sessionID, ticketID, scannerID string) (time.Time, bool, error) {
	var at time.Time
	err := r.db.QueryRow(ctx, `
		INSERT INTO scan_events (session_id, ticket_id, scanned_by) VALUES ($1, $2, $3)
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

func (r *Repository) GetScan(ctx context.Context, sessionID, ticketID string) (time.Time, *string, error) {
	var at time.Time
	var by *string
	err := r.db.QueryRow(ctx, `
		SELECT e.scanned_at, u.name FROM scan_events e LEFT JOIN users u ON u.id = e.scanned_by
		WHERE e.session_id = $1 AND e.ticket_id = $2`, sessionID, ticketID).Scan(&at, &by)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("get scan: %w", err)
	}
	return at, by, nil
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

// MarkTeamOfLeader marks every entered member of the team led by leaderID in the session.
// Returns nil when the scanned user does not lead a team. The leader's own scan is excluded from the counts.
func (r *Repository) MarkTeamOfLeader(ctx context.Context, sessionID, leaderID, scannerID string) (*TeamScanResult, error) {
	var teamID, teamName string
	err := r.db.QueryRow(ctx, `SELECT id, name FROM teams WHERE leader_id = $1`, leaderID).Scan(&teamID, &teamName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find led team: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT u.name, t.id, COALESCE(t.is_checked_in, false)
		FROM users u
		LEFT JOIN tickets t ON t.user_id = u.id
		WHERE u.team_id = $1 AND u.id <> $2 AND u.deleted_at IS NULL
		ORDER BY u.name`, teamID, leaderID)
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	type member struct {
		name      string
		ticketID  *string
		checkedIn bool
	}
	var members []member
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.name, &m.ticketID, &m.checkedIn); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		members = append(members, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	res := &TeamScanResult{TeamName: teamName, Skipped: []TeamScanSkip{}}
	for _, m := range members {
		switch {
		case m.ticketID == nil:
			res.Skipped = append(res.Skipped, TeamScanSkip{Name: m.name, Reason: "no_pass"})
		case !m.checkedIn:
			res.Skipped = append(res.Skipped, TeamScanSkip{Name: m.name, Reason: "not_checked_in"})
		default:
			_, recorded, err := r.RecordScan(ctx, sessionID, *m.ticketID, scannerID)
			if err != nil {
				return nil, err
			}
			if recorded {
				res.Marked++
			} else {
				res.Already++
			}
		}
	}
	return res, nil
}

// sessionPeople returns every attendee who has entered, with their scan time in the session (if any).
func (r *Repository) sessionPeople(ctx context.Context, sessionID string) ([]SessionPerson, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.name, u.email, u.phone_number, tm.name, e.scanned_at
		FROM tickets t
		JOIN users u ON u.id = t.user_id AND u.deleted_at IS NULL
		LEFT JOIN teams tm ON tm.id = u.team_id
		LEFT JOIN scan_events e ON e.ticket_id = t.id AND e.session_id = $1
		WHERE t.is_checked_in = true
		ORDER BY u.name`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query session people: %w", err)
	}
	defer rows.Close()
	var out []SessionPerson
	for rows.Next() {
		var p SessionPerson
		var at *time.Time
		if err := rows.Scan(&p.UserID, &p.Name, &p.Email, &p.Phone, &p.TeamName, &at); err != nil {
			return nil, fmt.Errorf("scan session person: %w", err)
		}
		if at != nil {
			s := at.Format(time.RFC3339)
			p.ScannedAt = &s
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
	markTeam := req.Kind == KindAttendance
	if req.MarkTeam != nil {
		markTeam = *req.MarkTeam && req.Kind == KindAttendance
	}
	sess, err := s.repo.CreateSession(ctx, req.Name, req.Kind, markTeam, actorID)
	if err != nil {
		return nil, err
	}
	s.recordAudit(ctx, audit.ActionScanSessionCreate, audit.TargetScanSession, &sess.ID,
		fmt.Sprintf("%s created %s scan session %q", actorEmail, req.Kind, req.Name),
		&actorID, &actorEmail, &actorRole, map[string]any{"name": req.Name, "kind": req.Kind}, &ip, &ua)
	return sess, nil
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

// ScanInSession validates and records an attendance or exit scan.
// On refusal it returns a rejection describing why, alongside the sentinel error.
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

	info, err := s.repo.GetTicketByCode(ctx, ticketCode)
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

	if !info.IsCheckedIn {
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
		prevAt, prevBy, err := s.repo.GetScan(ctx, sess.ID, info.TicketID)
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
			ScannedAt: prevAt.Format(time.RFC3339), ScannedByName: prevBy, Attendee: attendee,
		}, ErrAlreadyScanned
	}

	metrics.InstrumentTicketScan("valid")
	s.recordAudit(ctx, audit.ActionTicketSessionScan, audit.TargetTicket, &info.TicketID,
		fmt.Sprintf("%s scanned %s in %s %q", scannerEmail, info.UserEmail, sess.Kind, sess.Name),
		&scannerID, &scannerEmail, &scannerRole,
		map[string]any{"session_id": sess.ID, "session_name": sess.Name, "kind": sess.Kind, "attendee_email": info.UserEmail},
		&ip, &ua)

	resp := &SessionScanResponse{
		Success: true, Message: "Scan recorded", Session: *sess,
		ScannedAt: at.Format(time.RFC3339), Attendee: attendee,
	}

	if sess.Kind == KindAttendance && sess.MarkTeam {
		team, err := s.repo.MarkTeamOfLeader(ctx, sess.ID, info.UserID, scannerID)
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
				map[string]any{"team": team.TeamName, "marked": team.Marked, "already": team.Already, "skipped": len(team.Skipped)},
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
	people, err := s.repo.sessionPeople(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	domains, err := s.InternalDomains(ctx)
	if err != nil {
		return nil, err
	}
	report := &SessionReport{Session: *sess, Missing: []SessionPerson{}, Recent: []SessionPerson{}}
	for _, p := range people {
		p.External = !utils.IsEmailExcluded(p.Email, domains)
		if sess.Kind == KindExit && !p.External {
			continue // internal attendees are not expected to exit-scan
		}
		report.Eligible++
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

// ReleaseTickets creates tickets for matching users and shows them on the dashboard, sending no email.
func (s *Service) ReleaseTickets(ctx context.Context, req BulkSendTicketsRequest, actorID, actorEmail, actorRole, ip, ua string) (*ReleaseTicketsResponse, error) {
	eligible, err := s.repo.FindEligibleUsers(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &ReleaseTicketsResponse{TotalProcessed: 0}
	for _, u := range eligible {
		if utils.IsEmailExcluded(u.Email, req.ExcludeDomains) {
			continue
		}
		resp.TotalProcessed++

		ticketID := u.TicketID
		if u.TicketCode != "" {
			resp.AlreadyHad++
		} else {
			code, err := s.generateTicketCode()
			if err != nil {
				resp.Skipped++
				continue
			}
			t, err := s.repo.GetOrCreateTicket(ctx, u.UserID, code)
			if err != nil {
				resp.Skipped++
				continue
			}
			ticketID = t.ID
			resp.Released++
		}
		if err := s.repo.ReleaseTicket(ctx, ticketID); err != nil {
			resp.Skipped++
		}
	}
	s.recordAudit(ctx, audit.ActionTicketReleased, audit.TargetTicket, nil,
		fmt.Sprintf("%s released %d passes to the dashboard without email", actorEmail, resp.Released),
		&actorID, &actorEmail, &actorRole,
		map[string]any{"released": resp.Released, "already_had": resp.AlreadyHad, "skipped": resp.Skipped},
		&ip, &ua)
	return resp, nil
}
