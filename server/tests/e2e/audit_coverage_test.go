package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// waitForAudit blocks until at least min rows with the action exist (audit writes are async via RabbitMQ).
func waitForAudit(t *testing.T, action string, min int) {
	t.Helper()
	count := func() (int, error) {
		var n int
		err := testApp.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM audit_logs WHERE action = $1`, action).Scan(&n)
		return n, err
	}
	WaitForCondition(t, 10*time.Second, 100*time.Millisecond, "audit rows for "+action,
		func() (bool, error) { n, err := count(); return n >= min, err },
		func() string { n, _ := count(); return fmt.Sprintf("%s: %d rows, want >= %d", action, n, min) })
}

// Session scan refusals, duplicates, settings changes, release and password-reset requests all leave an audit trail.
func TestStage7_AuditTrailForTicketingAndAuth(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	var created struct {
		Users []struct {
			ID string `json:"id"`
		} `json:"users"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk", map[string]any{
		"users": []map[string]any{{"email": "aud@internal.example", "name": "Audit Ann", "role": "participant", "send_invite": false}},
	}, http.StatusCreated, &created)
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/release", map[string]any{"role": "participant"}, http.StatusOK, nil)
	var code string
	if err := testApp.DB.QueryRow(context.Background(),
		`SELECT t.ticket_code FROM tickets t JOIN users u ON u.id = t.user_id WHERE u.email = 'aud@internal.example'`).Scan(&code); err != nil {
		t.Fatal(err)
	}

	var att, exit struct {
		ID string `json:"id"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/sessions", map[string]string{"name": "A1", "kind": "attendance"}, http.StatusCreated, &att)
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/sessions", map[string]string{"name": "X1", "kind": "exit"}, http.StatusCreated, &exit)
	scan := func(id, c string, want int) {
		admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/tickets/sessions/%s/scan", id), map[string]string{"ticket_code": c}, want, nil)
	}

	scan(att.ID, code, http.StatusUnprocessableEntity) // not checked in -> scan_rejected
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/check-in", map[string]string{"ticket_code": code}, http.StatusOK, nil)
	scan(att.ID, code, http.StatusOK)
	scan(att.ID, code, http.StatusConflict)                          // duplicate
	scan(exit.ID, code, http.StatusUnprocessableEntity)              // internal at exit -> scan_rejected
	scan(att.ID, "cadence:tkt:forged:00000000", http.StatusNotFound) // unknown ticket -> scan_rejected
	admin.DoJSON(t, http.MethodPut, "/api/v1/tickets/settings", map[string]any{"internal_email_domains": []string{"internal.example"}}, http.StatusOK, nil)

	// Forgot-password: audited whether or not the account exists, while the response stays generic.
	anon := NewActor(t, "anon", "anon@nowhere.example")
	anon.DoJSON(t, http.MethodPost, "/api/v1/auth/forgot-password", map[string]string{"email": "aud@internal.example"}, http.StatusOK, nil)
	anon.DoJSON(t, http.MethodPost, "/api/v1/auth/forgot-password", map[string]string{"email": "ghost@nowhere.example"}, http.StatusOK, nil)

	waitForAudit(t, "ticket.scan_rejected", 3)
	waitForAudit(t, "ticket.duplicate_scan", 1)
	waitForAudit(t, "ticket.released", 1)
	waitForAudit(t, "ticket.session_create", 2)
	waitForAudit(t, "ticket.settings_update", 1)
	waitForAudit(t, "auth.password_forgot", 2)

	// The raw ticket code must never reach the audit trail.
	var leaked int
	if err := testApp.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM audit_logs WHERE description LIKE '%'||$1||'%' OR metadata::text LIKE '%'||$1||'%'`, code).Scan(&leaked); err != nil {
		t.Fatal(err)
	}
	if leaked != 0 {
		t.Fatalf("ticket code leaked into %d audit rows", leaked)
	}
}
