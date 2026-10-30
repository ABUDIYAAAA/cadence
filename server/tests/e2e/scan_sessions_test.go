package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/modules/tickets"
)

// Release shows passes without email; attendance rounds need entry; exit is for external attendees only.
func TestStage7_ScanSessions(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	var created struct {
		Users []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"users"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk", map[string]any{
		"users": []map[string]any{
			{"email": "ins@internal.example", "name": "Inside Ina", "role": "participant", "send_invite": false},
			{"email": "ext@outside.example", "name": "Outside Otto", "role": "participant", "send_invite": false},
		},
	}, http.StatusCreated, &created)

	// Release: tickets exist for both, nobody is emailed.
	var released tickets.ReleaseTicketsResponse
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/release", map[string]any{"role": "participant"}, http.StatusOK, &released)
	if released.Released != 2 {
		t.Fatalf("expected 2 passes released, got %+v", released)
	}
	time.Sleep(time.Second)
	if mailpitHasRecipient(t, "ext@outside.example") {
		t.Fatal("release must not send email")
	}

	code := func(email string) string {
		var c string
		err := testApp.DB.QueryRow(context.Background(),
			`SELECT t.ticket_code FROM tickets t JOIN users u ON u.id = t.user_id WHERE u.email = $1`, email).Scan(&c)
		if err != nil {
			t.Fatalf("ticket code for %s: %v", email, err)
		}
		return c
	}
	insCode, extCode := code("ins@internal.example"), code("ext@outside.example")

	var attendance, exit tickets.ScanSession
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/sessions", map[string]string{"name": "Night 1", "kind": "attendance"}, http.StatusCreated, &attendance)
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/sessions", map[string]string{"name": "Gate out", "kind": "exit"}, http.StatusCreated, &exit)
	scanPath := func(id string) string { return fmt.Sprintf("/api/v1/tickets/sessions/%s/scan", id) }

	// Not entered yet
	var rej tickets.SessionScanRejection
	admin.DoJSON(t, http.MethodPost, scanPath(attendance.ID), map[string]string{"ticket_code": insCode}, http.StatusUnprocessableEntity, &rej)
	if rej.Code != "not_checked_in" {
		t.Fatalf("expected not_checked_in, got %+v", rej)
	}

	for _, c := range []string{insCode, extCode} {
		admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/check-in", map[string]string{"ticket_code": c}, http.StatusOK, nil)
	}

	var ok tickets.SessionScanResponse
	admin.DoJSON(t, http.MethodPost, scanPath(attendance.ID), map[string]string{"ticket_code": insCode}, http.StatusOK, &ok)
	if !ok.Success || ok.Attendee.Email != "ins@internal.example" {
		t.Fatalf("unexpected attendance scan: %+v", ok)
	}
	admin.DoJSON(t, http.MethodPost, scanPath(attendance.ID), map[string]string{"ticket_code": insCode}, http.StatusConflict, &rej)
	if rej.Code != "duplicate" || rej.ScannedAt == "" {
		t.Fatalf("expected duplicate with prior scan, got %+v", rej)
	}

	// Exit: internal refused, external allowed
	admin.DoJSON(t, http.MethodPost, scanPath(exit.ID), map[string]string{"ticket_code": insCode}, http.StatusUnprocessableEntity, &rej)
	if rej.Code != "exit_not_required" {
		t.Fatalf("expected exit_not_required, got %+v", rej)
	}
	admin.DoJSON(t, http.MethodPost, scanPath(exit.ID), map[string]string{"ticket_code": extCode}, http.StatusOK, &ok)

	var report tickets.SessionReport
	admin.DoJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/tickets/sessions/%s/report", attendance.ID), nil, http.StatusOK, &report)
	if report.Eligible != 2 || report.Scanned != 1 || len(report.Missing) != 1 || report.Missing[0].Email != "ext@outside.example" {
		t.Fatalf("unexpected attendance report: %+v", report)
	}
	admin.DoJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/tickets/sessions/%s/report", exit.ID), nil, http.StatusOK, &report)
	if report.Eligible != 1 || report.Scanned != 1 || len(report.Missing) != 0 {
		t.Fatalf("unexpected exit report: %+v", report)
	}

	// Closed sessions refuse scans
	admin.DoJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/tickets/sessions/%s", attendance.ID), map[string]bool{"is_open": false}, http.StatusOK, nil)
	admin.DoJSON(t, http.MethodPost, scanPath(attendance.ID), map[string]string{"ticket_code": extCode}, http.StatusConflict, &rej)
	if rej.Code != "session_closed" {
		t.Fatalf("expected session_closed, got %+v", rej)
	}
}

// A team leader's attendance scan marks the whole team; members who never entered are reported, not marked.
func TestStage7_TeamLeaderScanMarksTeam(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	var created struct {
		Users []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"users"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk", map[string]any{
		"users": []map[string]any{
			{"email": "lead@internal.example", "name": "Lead Lia", "role": "participant", "send_invite": false},
			{"email": "m1@internal.example", "name": "Member One", "role": "participant", "send_invite": false},
			{"email": "m2@internal.example", "name": "Member Two", "role": "participant", "send_invite": false},
		},
	}, http.StatusCreated, &created)
	id := map[string]string{}
	for _, u := range created.Users {
		id[u.Email] = u.ID
	}
	lead := id["lead@internal.example"]
	admin.DoJSON(t, http.MethodPost, "/api/v1/teams", map[string]any{
		"name": "Squad", "leader_id": lead,
		"initial_member_ids": []string{id["m1@internal.example"], id["m2@internal.example"]},
	}, http.StatusCreated, nil)

	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/release", map[string]any{"role": "participant"}, http.StatusOK, nil)
	code := func(email string) string {
		var c string
		if err := testApp.DB.QueryRow(context.Background(),
			`SELECT t.ticket_code FROM tickets t JOIN users u ON u.id = t.user_id WHERE u.email = $1`, email).Scan(&c); err != nil {
			t.Fatalf("ticket code for %s: %v", email, err)
		}
		return c
	}
	// Leader and Member One entered; Member Two never did.
	for _, e := range []string{"lead@internal.example", "m1@internal.example"} {
		admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/check-in", map[string]string{"ticket_code": code(e)}, http.StatusOK, nil)
	}

	var sess tickets.ScanSession
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/sessions", map[string]string{"name": "Round 1", "kind": "attendance"}, http.StatusCreated, &sess)
	if !sess.MarkTeam {
		t.Fatalf("attendance sessions should mark teams by default: %+v", sess)
	}

	var resp tickets.SessionScanResponse
	scan := fmt.Sprintf("/api/v1/tickets/sessions/%s/scan", sess.ID)
	admin.DoJSON(t, http.MethodPost, scan, map[string]string{"ticket_code": code("lead@internal.example")}, http.StatusOK, &resp)
	if resp.Team == nil || resp.Team.Marked != 1 || len(resp.Team.Skipped) != 1 || resp.Team.Skipped[0].Reason != "not_checked_in" {
		t.Fatalf("unexpected team scan result: %+v", resp.Team)
	}
	if resp.Session.ScannedCount != 2 {
		t.Fatalf("expected 2 scans recorded (leader + 1 member), got %d", resp.Session.ScannedCount)
	}

	// The marked member is now a duplicate when scanned on their own.
	var rej tickets.SessionScanRejection
	admin.DoJSON(t, http.MethodPost, scan, map[string]string{"ticket_code": code("m1@internal.example")}, http.StatusConflict, &rej)
	if rej.Code != "duplicate" {
		t.Fatalf("expected duplicate for team-marked member, got %+v", rej)
	}
}

// Internal domains are editable at runtime and decide who must exit-scan.
func TestStage7_InternalDomainSettings(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	var s tickets.TicketSettings
	admin.DoJSON(t, http.MethodGet, "/api/v1/tickets/settings", nil, http.StatusOK, &s)
	if s.Source != "environment" || len(s.InternalEmailDomains) != 1 || s.InternalEmailDomains[0] != "internal.example" {
		t.Fatalf("expected env fallback, got %+v", s)
	}

	admin.DoJSON(t, http.MethodPut, "/api/v1/tickets/settings", map[string]any{
		"internal_email_domains": []string{"@Other.Example", "other.example", " "},
	}, http.StatusOK, &s)
	if s.Source != "database" || len(s.InternalEmailDomains) != 1 || s.InternalEmailDomains[0] != "other.example" {
		t.Fatalf("expected normalised saved list, got %+v", s)
	}
	admin.DoJSON(t, http.MethodPut, "/api/v1/tickets/settings", map[string]any{
		"internal_email_domains": []string{"not a domain"},
	}, http.StatusBadRequest, nil)

	// internal.example is no longer internal, so its attendees now exit-scan.
	var created struct {
		Users []struct {
			ID string `json:"id"`
		} `json:"users"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk", map[string]any{
		"users": []map[string]any{{"email": "was@internal.example", "name": "Was Internal", "role": "participant", "send_invite": false}},
	}, http.StatusCreated, &created)
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/release", map[string]any{"role": "participant"}, http.StatusOK, nil)
	var c string
	if err := testApp.DB.QueryRow(context.Background(),
		`SELECT t.ticket_code FROM tickets t JOIN users u ON u.id = t.user_id WHERE u.email = 'was@internal.example'`).Scan(&c); err != nil {
		t.Fatal(err)
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/check-in", map[string]string{"ticket_code": c}, http.StatusOK, nil)
	var exit tickets.ScanSession
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/sessions", map[string]string{"name": "Out", "kind": "exit"}, http.StatusCreated, &exit)
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/tickets/sessions/%s/scan", exit.ID), map[string]string{"ticket_code": c}, http.StatusOK, nil)
}
