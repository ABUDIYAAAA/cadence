package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"devclubxnst.com/cadence/internal/modules/judging"
	"devclubxnst.com/cadence/internal/modules/tickets"
)

// Submissions are closed until an admin picks an attendance session; then only teams with at
// least one member scanned in it can submit. Admins export every team's submission.
func TestSubmissions_AttendanceGate(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	aLead := CreateParticipant(t, admin, "a.lead@internal.example", "A Lead")
	aMember := CreateParticipant(t, admin, "a.m1@internal.example", "A Member")
	bLead := CreateParticipant(t, admin, "b.lead@internal.example", "B Lead")
	teamA := CreateTeamAs(t, aLead, "Team A")
	JoinTeamAs(t, aMember, teamA.JoinCode)
	CreateTeamAs(t, bLead, "Team B")

	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/release", map[string]any{"role": "participant"}, http.StatusOK, nil)
	code := func(email string) string {
		var c string
		if err := testApp.DB.QueryRow(context.Background(),
			`SELECT t.ticket_code FROM tickets t JOIN users u ON u.id = t.user_id WHERE u.email = $1`, email).Scan(&c); err != nil {
			t.Fatalf("ticket code for %s: %v", email, err)
		}
		return c
	}

	payload := map[string]any{
		"project_name": "Alpha App",
		"links":        map[string]string{"github": "https://github.com/cadence/alpha"},
	}
	var mine judging.MySubmission

	// No gate configured: closed for everyone.
	aLead.DoJSON(t, http.MethodGet, "/api/v1/judging/my-submission", nil, http.StatusOK, &mine)
	if mine.CanSubmit {
		t.Fatalf("submissions must be closed by default: %+v", mine)
	}
	aLead.DoJSON(t, http.MethodPut, "/api/v1/judging/my-submission", payload, http.StatusForbidden, nil)

	// Gate on an attendance session; only one member of Team A (not the leader) is scanned.
	var sess tickets.ScanSession
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/sessions", map[string]string{"name": "Gate", "kind": "attendance"}, http.StatusCreated, &sess)
	admin.DoJSON(t, http.MethodPut, "/api/v1/judging/submission-gate", map[string]any{"session_id": sess.ID}, http.StatusOK, nil)
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/tickets/sessions/%s/scan", sess.ID),
		map[string]string{"ticket_code": code("a.m1@internal.example")}, http.StatusOK, nil)

	aLead.DoJSON(t, http.MethodGet, "/api/v1/judging/my-submission", nil, http.StatusOK, &mine)
	if !mine.CanSubmit {
		t.Fatalf("team with one member present should be able to submit: %+v", mine)
	}
	aLead.DoJSON(t, http.MethodPut, "/api/v1/judging/my-submission", payload, http.StatusOK, nil)
	bLead.DoJSON(t, http.MethodPut, "/api/v1/judging/my-submission", payload, http.StatusForbidden, nil)

	// Export lists both teams; only Team A has a submission.
	var out judging.SubmissionsResponse
	admin.DoJSON(t, http.MethodGet, "/api/v1/judging/submissions", nil, http.StatusOK, &out)
	if len(out.Submissions) != 2 || out.Submissions[0].TeamName != "Team A" ||
		out.Submissions[0].ProjectName != "Alpha App" || out.Submissions[0].Repository != "https://github.com/cadence/alpha" ||
		out.Submissions[1].ProjectName != "" {
		t.Fatalf("unexpected submissions export: %+v", out.Submissions)
	}

	// Participants cannot read the export or change the gate.
	aLead.DoJSON(t, http.MethodGet, "/api/v1/judging/submissions", nil, http.StatusForbidden, nil)
	aLead.DoJSON(t, http.MethodPut, "/api/v1/judging/submission-gate", map[string]any{"session_id": nil}, http.StatusForbidden, nil)

	// Announcements by attendance: present = Team A's scanned member only.
	var resp struct {
		RecipientsCount int `json:"recipients_count"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/notifications/broadcast", map[string]any{
		"title": "T", "body": "B", "target_type": "attendance_present", "target_id": sess.ID,
	}, http.StatusCreated, &resp)
	if resp.RecipientsCount != 1 {
		t.Fatalf("expected 1 present recipient, got %d", resp.RecipientsCount)
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/notifications/broadcast", map[string]any{
		"title": "T", "body": "B", "target_type": "attendance_absent", "target_id": sess.ID,
	}, http.StatusCreated, &resp)
	if resp.RecipientsCount != 2 {
		t.Fatalf("expected 2 absent recipients, got %d", resp.RecipientsCount)
	}
}
