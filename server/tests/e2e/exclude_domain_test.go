package e2e_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/modules/tickets"
)

// mailpitHasRecipient reports whether any message in Mailpit was addressed to email.
func mailpitHasRecipient(t *testing.T, email string) bool {
	t.Helper()
	list, err := fetchMailpitMessages()
	if err != nil {
		t.Fatalf("fetch mailpit: %v", err)
	}
	for _, m := range list.Messages {
		for _, to := range m.To {
			if strings.EqualFold(to.Address, email) {
				return true
			}
		}
	}
	return false
}

// TestExcludeDomain_InvitesAndTickets verifies server-side exclude_domains on bulk invites and bulk tickets.
func TestExcludeDomain_InvitesAndTickets(t *testing.T) {
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
			{"email": "keep@real.example", "name": "Keep Me", "role": "participant", "send_invite": false},
			{"email": "skip@excluded.test", "name": "Skip Me", "role": "participant", "send_invite": false},
		},
	}, http.StatusCreated, &created)
	ids := map[string]string{}
	for _, u := range created.Users {
		ids[u.Email] = u.ID
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 created users, got %+v", created)
	}

	// --- Invites: excluded domain is never mailed ---
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk-send-invite", map[string]any{
		"all_uninvited":   true,
		"exclude_domains": []string{"@Excluded.test"},
	}, http.StatusOK, nil)
	WaitForEmail(t, "keep@real.example", "Set Up Your Password")

	// --- Tickets: excluded domain is never mailed ---
	var resp tickets.BulkSendTicketsResponse
	admin.DoJSON(t, http.MethodPost, "/api/v1/tickets/bulk-send", map[string]any{
		"role":            "participant",
		"exclude_domains": []string{"excluded.test"},
		"skip_checked_in": true,
	}, http.StatusOK, &resp)
	if resp.Enqueued != 1 {
		t.Fatalf("expected exactly 1 ticket enqueued, got %+v", resp)
	}
	_ = VerifyAndDownloadTicketEmail(t, "keep@real.example")

	time.Sleep(2 * time.Second)
	if mailpitHasRecipient(t, "skip@excluded.test") {
		t.Fatal("excluded-domain user received an email")
	}
}
