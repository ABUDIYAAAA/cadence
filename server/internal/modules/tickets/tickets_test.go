package tickets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/config"
	"devclubxnst.com/cadence/internal/modules/events"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func newTestDBPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:password@127.0.0.1:5434/lms?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping tickets db test: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping tickets db test, db unreachable: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})
	return pool
}

func seedTestEvent(t *testing.T, pool *pgxpool.Pool, slug, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO events (slug, name)
		VALUES ($1, $2)
		RETURNING id
	`, slug, name).Scan(&id); err != nil {
		t.Fatalf("failed to seed test event: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM events WHERE id = $1`, id)
	})
	return id
}

func seedTestEventUser(t *testing.T, pool *pgxpool.Pool, eventID, email, name, role string) string {
	t.Helper()
	ctx := context.Background()
	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, is_active)
		VALUES ($1, $2, 'user', TRUE)
		RETURNING id
	`, email, name).Scan(&userID); err != nil {
		t.Fatalf("failed to seed test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	if _, err := pool.Exec(ctx, `
		INSERT INTO user_events (event_id, user_id, role)
		VALUES ($1, $2, $3::event_role)
	`, eventID, userID, role); err != nil {
		t.Fatalf("failed to seed user_events row: %v", err)
	}
	return userID
}

func TestTicketSettingsSanitizationAndExcludedDomains(t *testing.T) {
	pool := newTestDBPool(t)

	ts := time.Now().UnixNano()
	evSlug := fmt.Sprintf("tkt-settings-ev-%d", ts)
	eventID := seedTestEvent(t, pool, evSlug, "Ticketing Settings Event")
	adminID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("admin_%d@example.com", ts), "Admin", "admin")

	cfg := &config.Config{
		CookieSecret: "test-ticket-cookie-secret-1234567890",
	}
	repo := NewRepository(pool)
	svc := NewService(repo, cfg, nil, nil)

	ctx := events.WithEventContext(context.Background(), &events.Context{
		EventID: eventID,
		Slug:    evSlug,
		Name:    "Ticketing Settings Event",
		UserID:  adminID,
		Role:    "admin",
	})

	subj := "Your Official Pass for {{event_name}}"
	unsafeHTML := `<h1>Hello {{name}}</h1><script>alert('xss')</script><p onload="evil()">Bring your ID to <strong>{{event_name}}</strong>.</p><a href="javascript:steal()">Bad</a>`

	updated, err := svc.UpdateSettings(ctx, UpdateTicketSettingsRequest{
		ExcludedDomains: []string{"@nst.rishihood.edu.in", "staff@internal.edu", "  INTERNAL.EDU "},
		EmailSubject:    &subj,
		EmailHTML:       &unsafeHTML,
	}, adminID, "admin@example.com")
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	if len(updated.ExcludedDomains) != 2 || updated.ExcludedDomains[0] != "internal.edu" || updated.ExcludedDomains[1] != "nst.rishihood.edu.in" {
		t.Fatalf("expected normalized excluded domains [internal.edu nst.rishihood.edu.in], got %v", updated.ExcludedDomains)
	}
	if strings.Contains(updated.EmailHTML, "<script") || strings.Contains(updated.EmailHTML, "onload") || strings.Contains(updated.EmailHTML, "javascript:") {
		t.Fatalf("expected unsafe HTML stripped in saved settings, got: %s", updated.EmailHTML)
	}
	if !strings.Contains(updated.EmailHTML, "<h1>Hello {{name}}</h1>") || !strings.Contains(updated.EmailHTML, "<strong>{{event_name}}</strong>") {
		t.Fatalf("expected safe HTML and placeholders preserved, got: %s", updated.EmailHTML)
	}
}

func TestBulkSendExcludedDomainsHardEmailTrackingAndLateJoinerVisibility(t *testing.T) {
	pool := newTestDBPool(t)

	ts := time.Now().UnixNano()
	evSlug := fmt.Sprintf("tkt-flow-ev-%d", ts)
	eventID := seedTestEvent(t, pool, evSlug, "Cadence Summit 2026")
	adminID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("admin_flow_%d@example.com", ts), "Admin", "admin")

	// Seed 2 internal users (on excluded @domains) and 2 external users
	internal1 := seedTestEventUser(t, pool, eventID, fmt.Sprintf("student1_%d@nst.rishihood.edu.in", ts), "Internal Student 1", "participant")
	internal2 := seedTestEventUser(t, pool, eventID, fmt.Sprintf("student2_%d@sub.college.edu", ts), "Internal Student 2", "participant")
	external1 := seedTestEventUser(t, pool, eventID, fmt.Sprintf("ext1_%d@gmail.com", ts), "External Guest 1", "participant")
	external2 := seedTestEventUser(t, pool, eventID, fmt.Sprintf("ext2_%d@outlook.com", ts), "External Guest 2", "participant")

	cfg := &config.Config{
		CookieSecret: "test-ticket-cookie-secret-1234567890",
	}
	repo := NewRepository(pool)
	svc := NewService(repo, cfg, nil, nil)

	ctx := events.WithEventContext(context.Background(), &events.Context{
		EventID: eventID,
		Slug:    evSlug,
		Name:    "Cadence Summit 2026",
		UserID:  adminID,
		Role:    "admin",
	})

	// Step 0: Before tickets are released, GetMyTicket returns ErrTicketNotFound
	if _, err := svc.GetMyTicket(ctx, external1); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound before tickets are released, got %v", err)
	}

	// Step 1: Configure excluded domains in settings (@nst.rishihood.edu.in) and custom HTML
	customHTML := `<p>Welcome {{name}} to {{event_name}}!</p><script>evil()</script>`
	if _, err := svc.UpdateSettings(ctx, UpdateTicketSettingsRequest{
		ExcludedDomains: []string{"@nst.rishihood.edu.in"},
		EmailHTML:       &customHTML,
	}, adminID, "admin@example.com"); err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	// Step 2: Run BulkSendTickets with an additional request-level excluded domain ("@college.edu")
	// Total eligible users in event = 5 (admin + 2 internal + 2 external).
	// Filter to role="participant" so we process the 4 participants.
	partRole := "participant"
	bulkResp1, err := svc.BulkSendTickets(ctx, BulkSendTicketsRequest{
		Role:           &partRole,
		ExcludeDomains: []string{"@college.edu"},
	}, adminID, "admin@example.com")
	if err != nil {
		t.Fatalf("BulkSendTickets run 1 failed: %v", err)
	}

	if bulkResp1.TotalProcessed != 4 {
		t.Fatalf("expected TotalProcessed=4, got %d", bulkResp1.TotalProcessed)
	}
	if bulkResp1.ExcludedDomain != 2 {
		t.Fatalf("expected ExcludedDomain=2 (the 2 internal students), got %d", bulkResp1.ExcludedDomain)
	}
	if bulkResp1.Enqueued != 2 {
		t.Fatalf("expected Enqueued=2 (the 2 external participants), got %d", bulkResp1.Enqueued)
	}
	if bulkResp1.AlreadyMailed != 0 {
		t.Fatalf("expected AlreadyMailed=0 on first run, got %d", bulkResp1.AlreadyMailed)
	}

	// Step 3: Verify excluded-domain internal users STILL have their ticket visible on dashboard, with email_status='skipped_domain'
	for _, uid := range []string{internal1, internal2} {
		myTkt, err := svc.GetMyTicket(ctx, uid)
		if err != nil {
			t.Fatalf("expected excluded-domain participant %s to still see their dashboard ticket, got error: %v", uid, err)
		}
		if myTkt.TicketCode == "" || myTkt.QRCodeBase64 == "" {
			t.Fatalf("expected valid ticket code and QR for internal participant %s", uid)
		}
		if myTkt.EmailStatus != EmailStatusSkippedDomain {
			t.Fatalf("expected EmailStatus=%q for internal user, got %q", EmailStatusSkippedDomain, myTkt.EmailStatus)
		}
	}

	// Step 4: Verify external users have email_status='sent' and EmailSentAt != nil
	for _, uid := range []string{external1, external2} {
		myTkt, err := svc.GetMyTicket(ctx, uid)
		if err != nil {
			t.Fatalf("expected external participant %s to see their ticket: %v", uid, err)
		}
		if myTkt.EmailStatus != EmailStatusSent || myTkt.EmailSentAt == nil {
			t.Fatalf("expected EmailStatus='sent' and non-nil EmailSentAt for external user, got status=%q sentAt=%v", myTkt.EmailStatus, myTkt.EmailSentAt)
		}
	}

	// Step 5: Run BulkSendTickets a second time -> external users must NOT be emailed again (hard tracking)
	bulkResp2, err := svc.BulkSendTickets(ctx, BulkSendTicketsRequest{
		Role:           &partRole,
		ExcludeDomains: []string{"@college.edu"},
	}, adminID, "admin@example.com")
	if err != nil {
		t.Fatalf("BulkSendTickets run 2 failed: %v", err)
	}
	if bulkResp2.Enqueued != 0 {
		t.Fatalf("expected Enqueued=0 on second run (all external users already mailed), got %d", bulkResp2.Enqueued)
	}
	if bulkResp2.AlreadyMailed != 2 {
		t.Fatalf("expected AlreadyMailed=2 on second run, got %d", bulkResp2.AlreadyMailed)
	}
	if bulkResp2.ExcludedDomain != 2 {
		t.Fatalf("expected ExcludedDomain=2 on second run, got %d", bulkResp2.ExcludedDomain)
	}

	// Step 6: Simulate a failed email delivery on external2, then re-run BulkSendTickets -> only external2 is retried!
	tktExt2, err := repo.GetTicketByUserID(ctx, external2)
	if err != nil {
		t.Fatalf("get external2 ticket: %v", err)
	}
	_, _ = pool.Exec(ctx, `UPDATE tickets SET email_status = 'failed', email_sent_at = NULL, email_error = 'smtp timeout' WHERE id = $1`, tktExt2.ID)

	bulkResp3, err := svc.BulkSendTickets(ctx, BulkSendTicketsRequest{
		Role:           &partRole,
		ExcludeDomains: []string{"@college.edu"},
	}, adminID, "admin@example.com")
	if err != nil {
		t.Fatalf("BulkSendTickets run 3 failed: %v", err)
	}
	if bulkResp3.Enqueued != 1 || bulkResp3.AlreadyMailed != 1 {
		t.Fatalf("expected failed user retried (Enqueued=1) and sent user skipped (AlreadyMailed=1), got %+v", bulkResp3)
	}

	// Step 7: ForceResend=true re-emails even already-sent external users (while still respecting excluded domains)
	bulkResp4, err := svc.BulkSendTickets(ctx, BulkSendTicketsRequest{
		Role:           &partRole,
		ExcludeDomains: []string{"@college.edu"},
		ForceResend:    true,
	}, adminID, "admin@example.com")
	if err != nil {
		t.Fatalf("BulkSendTickets run 4 (ForceResend) failed: %v", err)
	}
	if bulkResp4.Enqueued != 2 || bulkResp4.ExcludedDomain != 2 {
		t.Fatalf("expected Enqueued=2 and ExcludedDomain=2 with ForceResend=true, got %+v", bulkResp4)
	}

	// Step 8: Late-joining participant creates their account AFTER tickets were released -> GET /tickets/me works immediately!
	lateParticipantID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("late_user_%d@gmail.com", ts), "Late Joiner", "participant")
	lateTicket, err := svc.GetMyTicket(ctx, lateParticipantID)
	if err != nil {
		t.Fatalf("expected late-joining participant to automatically receive their released ticket on GetMyTicket, got: %v", err)
	}
	if lateTicket.TicketCode == "" || lateTicket.QRCodeBase64 == "" || lateTicket.ReleasedAt == nil {
		t.Fatalf("expected complete auto-minted ticket for late participant, got: %+v", lateTicket)
	}

	// Verify non-member of the event cannot auto-mint a ticket even when event tickets are released
	var outsiderID string
	_ = pool.QueryRow(ctx, `INSERT INTO users (email, name, role, is_active) VALUES ($1, 'Outsider', 'user', TRUE) RETURNING id`,
		fmt.Sprintf("outsider_%d@gmail.com", ts)).Scan(&outsiderID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, outsiderID)
	})
	if _, err := svc.GetMyTicket(ctx, outsiderID); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected outsider (not in user_events) to receive ErrTicketNotFound, got: %v", err)
	}
}

func TestDefaultCheckInAndCompetitionScanSessionsPerMemberAndFullTeam(t *testing.T) {
	pool := newTestDBPool(t)

	ts := time.Now().UnixNano()
	evSlug := fmt.Sprintf("scan-ev-%d", ts)
	eventID := seedTestEvent(t, pool, evSlug, "Scan Sessions Summit")

	ownerID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("owner_scan_%d@example.com", ts), "Event Owner", "owner")
	volunteerID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("vol_scan_%d@example.com", ts), "Volunteer Bob", "volunteer")
	leaderID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("leader_scan_%d@example.com", ts), "Alice Leader", "participant")
	member1ID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("m1_scan_%d@example.com", ts), "Charlie Member", "participant")
	member2ID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("m2_scan_%d@example.com", ts), "Dana Member", "participant")

	ctx := events.WithEventContext(context.Background(), &events.Context{
		EventID: eventID,
		Slug:    evSlug,
		Name:    "Scan Sessions Summit",
		UserID:  ownerID,
		Role:    "owner",
	})

	// Create Competition A in this event and a team led by leaderID with member1ID & member2ID
	var compAID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO competitions (event_id, slug, name)
		VALUES ($1, $2, 'Hackathon Track A')
		RETURNING id
	`, eventID, fmt.Sprintf("comp-a-%d", ts)).Scan(&compAID); err != nil {
		t.Fatalf("seed competition A: %v", err)
	}

	var teamID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO teams (competition_id, name, join_code, leader_id, created_by)
		VALUES ($1, $2, $3, $4, $4)
		RETURNING id
	`, compAID, fmt.Sprintf("Team Alpha %d", ts), fmt.Sprintf("ALP%05d", ts%100000), leaderID).Scan(&teamID); err != nil {
		t.Fatalf("seed team: %v", err)
	}
	for _, uid := range []string{leaderID, member1ID, member2ID} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO team_members (team_id, user_id, competition_id)
			VALUES ($1, $2, $3)
		`, teamID, uid, compAID); err != nil {
			t.Fatalf("seed team_member: %v", err)
		}
	}

	cfg := &config.Config{
		CookieSecret: "test-ticket-cookie-secret-1234567890",
	}
	repo := NewRepository(pool)
	svc := NewService(repo, cfg, nil, nil)

	// Release tickets so participants have passes
	if _, err := svc.ReleaseTickets(ctx, BulkSendTicketsRequest{}, ownerID, "owner@example.com"); err != nil {
		t.Fatalf("ReleaseTickets: %v", err)
	}
	leaderTkt, err := svc.GetMyTicket(ctx, leaderID)
	if err != nil {
		t.Fatalf("GetMyTicket leader: %v", err)
	}

	// 1. Verify default Check-In scan session exists automatically for the event
	sessionsList, err := svc.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessionsList) == 0 || !sessionsList[0].IsDefault || sessionsList[0].Kind != KindCheckIn || sessionsList[0].MarkTeam {
		t.Fatalf("expected first session to be default Entry Check-In (per-member), got: %+v", sessionsList)
	}
	defaultCheckInSession := sessionsList[0]

	// 2. Volunteer scans leader into the default Check-In session -> marks only leader (per-member) and records volunteer attribution
	checkInScanResp, rejection, err := svc.ScanInSession(
		ctx, defaultCheckInSession.ID, leaderTkt.TicketCode,
		volunteerID, "vol@example.com",
	)
	if err != nil || rejection != nil {
		t.Fatalf("default check-in scan failed: err=%v rejection=%+v", err, rejection)
	}
	if checkInScanResp.ScannedByID != volunteerID || checkInScanResp.ScannedByName != "Volunteer Bob" || checkInScanResp.ScannedAt == "" {
		t.Fatalf("expected scanner attribution on scan response, got: %+v", checkInScanResp)
	}
	// Verify leader's ticket is now marked is_checked_in=true
	leaderTktAfterCheckIn, _ := svc.GetMyTicket(ctx, leaderID)
	if !leaderTktAfterCheckIn.IsCheckedIn {
		t.Fatal("expected leader ticket is_checked_in=true after scanning in default check-in session")
	}

	// 3. Full-team-via-leader scan session requires a single competition_id
	markTeamTrue := true
	if _, err := svc.CreateSession(ctx, CreateSessionRequest{
		Name:     "Team Pitch Check",
		Kind:     KindAttendance,
		MarkTeam: &markTeamTrue,
	}, ownerID, "owner@example.com"); !errors.Is(err, ErrCompetitionRequiredForTeamScan) {
		t.Fatalf("expected ErrCompetitionRequiredForTeamScan when mark_team=true without competition_id, got: %v", err)
	}

	// 4. Create a Per-Member custom scan session (mark_team=false)
	markTeamFalse := false
	perMemberSess, err := svc.CreateSession(ctx, CreateSessionRequest{
		Name:          "Lunch Distribution (Per Member)",
		Kind:          KindAttendance,
		MarkTeam:      &markTeamFalse,
		CompetitionID: &compAID,
	}, ownerID, "owner@example.com")
	if err != nil {
		t.Fatalf("CreateSession per-member failed: %v", err)
	}
	if perMemberSess.MarkTeam {
		t.Fatal("expected perMemberSess.MarkTeam to be false")
	}

	// Volunteer scans leader in Per-Member session -> only 1 person (the leader) is scanned
	pmResp, _, err := svc.ScanInSession(
		ctx, perMemberSess.ID, leaderTkt.TicketCode,
		volunteerID, "vol@example.com",
	)
	if err != nil {
		t.Fatalf("per-member scan failed: %v", err)
	}
	if pmResp.Team != nil || pmResp.Session.ScannedCount != 1 {
		t.Fatalf("expected per-member scan to only mark 1 person without team cascade, got count=%d team=%+v", pmResp.Session.ScannedCount, pmResp.Team)
	}

	// 5. Create a Full-Team-via-Leader scan session linked to Competition A (mark_team=true)
	teamSess, err := svc.CreateSession(ctx, CreateSessionRequest{
		Name:          "Round 1 Team Attendance",
		Kind:          KindAttendance,
		MarkTeam:      &markTeamTrue,
		CompetitionID: &compAID,
	}, ownerID, "owner@example.com")
	if err != nil {
		t.Fatalf("CreateSession full-team failed: %v", err)
	}
	if !teamSess.MarkTeam || teamSess.CompetitionID == nil || *teamSess.CompetitionID != compAID {
		t.Fatalf("expected teamSess linked to compAID with MarkTeam=true, got: %+v", teamSess)
	}

	// Volunteer scans leader in Full-Team session -> leader + both teammates (3 total) are marked present with volunteer attribution!
	ftResp, _, err := svc.ScanInSession(
		ctx, teamSess.ID, leaderTkt.TicketCode,
		volunteerID, "vol@example.com",
	)
	if err != nil {
		t.Fatalf("full-team scan failed: %v", err)
	}
	if ftResp.Team == nil || ftResp.Team.Marked != 2 || ftResp.Session.ScannedCount != 3 {
		t.Fatalf("expected leader scan to mark 2 teammates (3 total), got count=%d team=%+v", ftResp.Session.ScannedCount, ftResp.Team)
	}

	// 6. Verify SessionReport records who was scanned by whom and when for all 3 team members
	report, err := svc.GetSessionReport(ctx, teamSess.ID)
	if err != nil {
		t.Fatalf("GetSessionReport failed: %v", err)
	}
	if report.Scanned != 3 {
		t.Fatalf("expected report.Scanned=3, got %d", report.Scanned)
	}
	for _, person := range report.People {
		if person.UserID == leaderID || person.UserID == member1ID || person.UserID == member2ID {
			if !person.Present || person.ScannedAt == nil || person.ScannedByID == nil || *person.ScannedByID != volunteerID {
				t.Fatalf("expected team member %s (%s) to have Present=true, ScannedAt, and ScannedByID=%s, got: %+v",
					person.Name, person.UserID, volunteerID, person)
			}
			if person.ScannedByName == nil || *person.ScannedByName != "Volunteer Bob" {
				t.Fatalf("expected ScannedByName='Volunteer Bob', got %v", person.ScannedByName)
			}
		}
	}
}

func TestCheckInCodeGenerationRotationAndDirectParticipantCheckIn(t *testing.T) {
	pool := newTestDBPool(t)

	ts := time.Now().UnixNano()
	evSlug := fmt.Sprintf("code-ev-%d", ts)
	eventID := seedTestEvent(t, pool, evSlug, "Code Check-In Event")

	adminID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("admin_code_%d@example.com", ts), "Admin", "admin")
	p1ID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("p1_code_%d@example.com", ts), "Participant 1", "participant")
	p2ID := seedTestEventUser(t, pool, eventID, fmt.Sprintf("p2_code_%d@example.com", ts), "Participant 2", "participant")

	ctx := events.WithEventContext(context.Background(), &events.Context{
		EventID: eventID,
		Slug:    evSlug,
		Name:    "Code Check-In Event",
		UserID:  adminID,
		Role:    "admin",
	})

	cfg := &config.Config{
		CookieSecret: "test-ticket-cookie-secret-1234567890",
	}
	repo := NewRepository(pool)
	svc := NewService(repo, cfg, nil, nil)

	// 1. Admin generates a check-in code on the default check-in session (Day 1 code)
	sess1, err := svc.GenerateSessionCheckInCode(ctx, "", nil, "admin@example.com")
	if err != nil {
		t.Fatalf("GenerateSessionCheckInCode 1 failed: %v", err)
	}
	if sess1.CheckInCode == nil || len(*sess1.CheckInCode) != 6 {
		t.Fatalf("expected 6-char random check-in code, got %v", sess1.CheckInCode)
	}
	codeDay1 := *sess1.CheckInCode

	// 2. Participant 1 enters the Day 1 check-in code directly (even before opening /tickets/me!)
	scanResp1, rejection, err := svc.CheckInByCode(ctx, strings.ToLower(codeDay1), p1ID, "p1@example.com")
	if err != nil || rejection != nil {
		t.Fatalf("CheckInByCode with codeDay1 failed: err=%v rejection=%+v", err, rejection)
	}
	if !scanResp1.Success || scanResp1.Attendee.ID != p1ID || scanResp1.ScannedByID != p1ID {
		t.Fatalf("unexpected scan response: %+v", scanResp1)
	}

	// Verify Participant 1's ticket is marked is_checked_in=true
	p1Tkt, err := svc.GetMyTicket(ctx, p1ID)
	if err != nil || !p1Tkt.IsCheckedIn {
		t.Fatalf("expected Participant 1 ticket to be checked in, got tkt=%+v err=%v", p1Tkt, err)
	}

	// 3. Admin generates another code (rotates the code on the session)
	sess2, err := svc.GenerateSessionCheckInCode(ctx, sess1.ID, nil, "admin@example.com")
	if err != nil {
		t.Fatalf("GenerateSessionCheckInCode 2 failed: %v", err)
	}
	if sess2.CheckInCode == nil || *sess2.CheckInCode == codeDay1 {
		t.Fatalf("expected rotated code to differ from %s, got %v", codeDay1, sess2.CheckInCode)
	}
	codeRotated := *sess2.CheckInCode

	// 4. Old code (codeDay1) is now invalid when Participant 2 tries to use it!
	if _, _, err := svc.CheckInByCode(ctx, codeDay1, p2ID, "p2@example.com"); !errors.Is(err, ErrInvalidCheckInCode) {
		t.Fatalf("expected old code %s to fail with ErrInvalidCheckInCode after rotation, got %v", codeDay1, err)
	}

	// 5. Participant 2 uses the new code (codeRotated) and succeeds!
	scanResp2, rejection, err := svc.CheckInByCode(ctx, codeRotated, p2ID, "p2@example.com")
	if err != nil || rejection != nil {
		t.Fatalf("CheckInByCode with rotated code failed: err=%v rejection=%+v", err, rejection)
	}
	if !scanResp2.Success || scanResp2.Attendee.ID != p2ID {
		t.Fatalf("unexpected scanResp2: %+v", scanResp2)
	}

	// 6. Admin can also create a separate Day 2 scan session with its own code so Participant 1 can check in for Day 2 as well!
	day2Sess, err := svc.CreateSession(ctx, CreateSessionRequest{
		Name:         "Day 2 Attendance",
		Kind:         KindAttendance,
		GenerateCode: true,
	}, adminID, "admin@example.com")
	if err != nil || day2Sess.CheckInCode == nil {
		t.Fatalf("CreateSession Day 2 with GenerateCode=true failed: sess=%+v err=%v", day2Sess, err)
	}

	scanRespDay2, rejection, err := svc.CheckInByCode(ctx, *day2Sess.CheckInCode, p1ID, "p1@example.com")
	if err != nil || rejection != nil {
		t.Fatalf("Participant 1 Day 2 code check-in failed: err=%v rejection=%+v", err, rejection)
	}
	if scanRespDay2.Session.ID != day2Sess.ID {
		t.Fatalf("expected scan recorded in Day 2 session %s, got %s", day2Sess.ID, scanRespDay2.Session.ID)
	}

	// 7. Verify GET /tickets/me lists both the Entry Check-In session and the Day 2 Attendance session in AttendedSessions
	p1AfterDay2, err := svc.GetMyTicket(ctx, p1ID)
	if err != nil {
		t.Fatalf("GetMyTicket after Day 2 code check-in failed: %v", err)
	}
	if len(p1AfterDay2.AttendedSessions) != 2 {
		t.Fatalf("expected 2 attended sessions (Entry Check-In + Day 2 Attendance), got %d: %+v", len(p1AfterDay2.AttendedSessions), p1AfterDay2.AttendedSessions)
	}
}
