package e2e_test

import (
	"fmt"
	"net/http"
	"testing"

	"devclubxnst.com/cadence/internal/modules/judging"
	"devclubxnst.com/cadence/internal/modules/users"
)

func TestStage4_RBACPermissionMatrix(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	// Create and onboard Judge1, Judge2 (unassigned), Volunteer, and Participant
	var bulkResp users.BulkCreateResponse
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk", map[string]any{
		"users": []map[string]any{
			{"email": "rbac.judge1@cadence.local", "name": "RBAC Judge 1", "role": "judge", "send_invite": true},
			{"email": "rbac.judge2@cadence.local", "name": "RBAC Judge 2", "role": "judge", "send_invite": true},
			{"email": "rbac.volunteer@cadence.local", "name": "RBAC Volunteer", "role": "volunteer", "send_invite": true},
			{"email": "rbac.participant@cadence.local", "name": "RBAC Participant", "role": "participant", "team_name": "RBAC Team", "is_leader": true, "send_invite": true},
		},
	}, http.StatusCreated, &bulkResp)

	judge1 := OnboardUserViaMailpit(t, "RBACJudge1", "rbac.judge1@cadence.local", "JudgePass123!")
	judge2Unassigned := OnboardUserViaMailpit(t, "RBACJudge2", "rbac.judge2@cadence.local", "JudgePass123!")
	volunteer := OnboardUserViaMailpit(t, "RBACVolunteer", "rbac.volunteer@cadence.local", "VolPass123!")
	participant := OnboardUserViaMailpit(t, "RBACParticipant", "rbac.participant@cadence.local", "PartPass123!")

	// Set up a round, panel (assigned ONLY to judge1), and slot for RBAC Team
	var teamResp struct {
		ID string `json:"id"`
	}
	participant.DoJSON(t, http.MethodGet, "/api/v1/teams/my-team", nil, http.StatusOK, &teamResp)

	var roundResp struct {
		RoundID string `json:"round_id"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/judging/rounds", map[string]any{
		"name":               "RBAC Round",
		"buffer_size":        1,
		"slot_duration_mins": 10,
		"criteria": []map[string]any{
			{"name": "Tech", "max_score": 10, "weight": 1.0, "order_index": 1},
		},
	}, http.StatusCreated, &roundResp)

	var panelResp struct {
		PanelID string `json:"panel_id"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/judging/panels", map[string]any{
		"round_id":  roundResp.RoundID,
		"name":      "RBAC Panel",
		"room":      "Room 202",
		"judge_ids": []string{judge1.UserID},
	}, http.StatusCreated, &panelResp)

	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/panels/%s/slots", panelResp.PanelID), map[string]any{
		"slots": []map[string]any{
			{"team_id": teamResp.ID, "sequence_order": 1},
		},
	}, http.StatusOK, nil)

	// Judges only see panels of rounds that have been activated
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/rounds/%s/activate", roundResp.RoundID), nil, http.StatusOK, nil)

	var judgePanels []judging.JudgePanelSummary
	judge1.DoJSON(t, http.MethodGet, "/api/v1/judging/judge/panels", nil, http.StatusOK, &judgePanels)
	slotID := judgePanels[0].Slots[0].SlotID
	critID := judgePanels[0].Criteria[0].ID

	// Table-driven RBAC forbidden checks across roles
	forbiddenCases := []struct {
		name   string
		actor  *Actor
		method string
		path   string
		body   any
	}{
		// Participant forbidden routes
		{"Participant cannot list users", participant, http.MethodGet, "/api/v1/users", nil},
		{"Participant cannot create user", participant, http.MethodPost, "/api/v1/users", map[string]any{"email": "x@x.com", "name": "X"}},
		{"Participant cannot list all teams", participant, http.MethodGet, "/api/v1/teams", nil},
		{"Participant cannot create team", participant, http.MethodPost, "/api/v1/teams", map[string]any{"name": "Illegal Team"}},
		{"Participant cannot scan tickets", participant, http.MethodPost, "/api/v1/tickets/check-in", map[string]string{"ticket_code": "cadence:tkt:123:456"}},
		{"Participant cannot view ticket analytics", participant, http.MethodGet, "/api/v1/tickets/analytics", nil},
		{"Participant cannot bulk send tickets", participant, http.MethodPost, "/api/v1/tickets/bulk-send", map[string]any{}},
		{"Participant cannot view volunteer schedule", participant, http.MethodGet, "/api/v1/judging/volunteer/schedule", nil},
		{"Participant cannot view judge panels", participant, http.MethodGet, "/api/v1/judging/judge/panels", nil},
		{"Participant cannot list judging rounds", participant, http.MethodGet, "/api/v1/judging/rounds", nil},
		{"Participant cannot view leaderboard", participant, http.MethodGet, fmt.Sprintf("/api/v1/judging/rounds/%s/leaderboard", roundResp.RoundID), nil},
		{"Participant cannot broadcast notifications", participant, http.MethodPost, "/api/v1/notifications/broadcast", map[string]any{"title": "T", "body": "B", "target_type": "broadcast"}},
		{"Participant cannot view audit logs", participant, http.MethodGet, "/api/v1/audit-logs", nil},

		// Volunteer forbidden routes (can read users/teams/rounds & scan tickets, but cannot mutate admin resources or judge)
		{"Volunteer cannot create user", volunteer, http.MethodPost, "/api/v1/users", map[string]any{"email": "v@x.com", "name": "V"}},
		{"Volunteer cannot delete user", volunteer, http.MethodDelete, fmt.Sprintf("/api/v1/users/%s", participant.UserID), nil},
		{"Volunteer cannot create team", volunteer, http.MethodPost, "/api/v1/teams", map[string]any{"name": "Vol Team"}},
		{"Volunteer cannot bulk send tickets", volunteer, http.MethodPost, "/api/v1/tickets/bulk-send", map[string]any{}},
		{"Volunteer cannot start judging slot", volunteer, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/start", slotID), nil},
		{"Volunteer cannot submit judging scores", volunteer, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/scores", slotID), map[string]any{"scores": []map[string]any{{"criterion_id": critID, "score": 5}}}},
		{"Volunteer cannot view admin leaderboard", volunteer, http.MethodGet, fmt.Sprintf("/api/v1/judging/rounds/%s/leaderboard", roundResp.RoundID), nil},
		{"Volunteer cannot broadcast notifications", volunteer, http.MethodPost, "/api/v1/notifications/broadcast", map[string]any{"title": "T", "body": "B", "target_type": "broadcast"}},
		{"Volunteer cannot view audit logs", volunteer, http.MethodGet, "/api/v1/audit-logs", nil},

		// Judge forbidden routes (cannot access admin-only or unassigned panel slots)
		{"Judge cannot create user", judge1, http.MethodPost, "/api/v1/users", map[string]any{"email": "j@x.com", "name": "J"}},
		{"Judge cannot create judging round", judge1, http.MethodPost, "/api/v1/judging/rounds", map[string]any{"name": "R", "criteria": []map[string]any{{"name": "C", "max_score": 10, "weight": 1}}}},
		{"Judge cannot view admin leaderboard", judge1, http.MethodGet, fmt.Sprintf("/api/v1/judging/rounds/%s/leaderboard", roundResp.RoundID), nil},
		{"Judge cannot view audit logs", judge1, http.MethodGet, "/api/v1/audit-logs", nil},
		{"Unassigned Judge cannot view slot details", judge2Unassigned, http.MethodGet, fmt.Sprintf("/api/v1/judging/slots/%s", slotID), nil},
		{"Unassigned Judge cannot start slot", judge2Unassigned, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/start", slotID), nil},
		{"Unassigned Judge cannot submit scores", judge2Unassigned, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/scores", slotID), map[string]any{"scores": []map[string]any{{"criterion_id": critID, "score": 8}}}},
		{"Unassigned Judge cannot lock scores", judge2Unassigned, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/lock", slotID), nil},
	}

	for _, tc := range forbiddenCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.actor.DoJSON(t, tc.method, tc.path, tc.body, http.StatusForbidden, nil)
		})
	}
}

func TestStage4_TokenReplayAndDeviceHijackProtection(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	// Create and onboard user
	admin.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email":       "sec.user@cadence.local",
		"name":        "Security User",
		"role":        "participant",
		"send_invite": true,
	}, http.StatusCreated, nil)

	user := OnboardUserViaMailpit(t, "SecUser", "sec.user@cadence.local", "SecPass123!")

	// 1. Token Rotation & Replay Detection
	initialRefreshCookie := user.GetCookieValue(t, "refresh_token")
	if initialRefreshCookie == "" {
		t.Fatal("expected initial refresh_token cookie")
	}

	// Rotate token once -> 200 OK
	user.DoJSON(t, http.MethodPost, "/api/v1/auth/refresh", nil, http.StatusOK, nil)
	rotatedRefreshCookie := user.GetCookieValue(t, "refresh_token")
	if rotatedRefreshCookie == "" || rotatedRefreshCookie == initialRefreshCookie {
		t.Fatal("expected refresh_token cookie to rotate to a new value")
	}

	// Replay the old (already used) refresh_token on the same device -> 401 Unauthorized (ErrTokenReplayDetect)
	user.SetCookieValue(t, "refresh_token", initialRefreshCookie)
	user.DoJSON(t, http.MethodPost, "/api/v1/auth/refresh", nil, http.StatusUnauthorized, nil)

	// Because replay compromised the session family, even the newly rotated refresh token must now be revoked!
	user.SetCookieValue(t, "refresh_token", rotatedRefreshCookie)
	user.DoJSON(t, http.MethodPost, "/api/v1/auth/refresh", nil, http.StatusUnauthorized, nil)

	// 2. Device Binding Anti-Hijack Protection
	// Log back in on Device A
	deviceA := NewActor(t, "DeviceA", "sec.user@cadence.local")
	deviceA.DoJSON(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "sec.user@cadence.local",
		"password": "SecPass123!",
	}, http.StatusOK, nil)

	stolenRefreshCookie := deviceA.GetCookieValue(t, "refresh_token")

	// Attacker on Device B (different _did cookie) steals the refresh_token and attempts /auth/refresh
	deviceB := NewActor(t, "DeviceB", "sec.user@cadence.local")
	// Trigger a request on Device B so it receives its own distinct signed _did cookie
	deviceB.DoJSON(t, http.MethodGet, "/health", nil, http.StatusOK, nil)
	deviceB.SetCookieValue(t, "refresh_token", stolenRefreshCookie)

	// Device mismatch must be rejected with 401 Unauthorized and revoke all sessions for the user
	deviceB.DoJSON(t, http.MethodPost, "/api/v1/auth/refresh", nil, http.StatusUnauthorized, nil)

	// Verify Device A's refresh token was also revoked as a protective measure
	deviceA.DoJSON(t, http.MethodPost, "/api/v1/auth/refresh", nil, http.StatusUnauthorized, nil)
}

func TestStage4_InstantRedisBlocklistOnDelete(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	admin.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email":       "to.delete@cadence.local",
		"name":        "Delete Target",
		"role":        "participant",
		"send_invite": true,
	}, http.StatusCreated, nil)

	target := OnboardUserViaMailpit(t, "DeleteTarget", "to.delete@cadence.local", "TargetPass123!")

	// Confirm target's access_token works before deletion
	target.DoJSON(t, http.MethodGet, "/api/v1/auth/me", nil, http.StatusOK, nil)

	// Admin deletes target user -> writes blocked:user:{id} to Redis
	admin.DoJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/users/%s", target.UserID), nil, http.StatusOK, nil)

	// Target's non-expired access_token must be rejected immediately (<1ms) with 401 Unauthorized
	target.DoJSON(t, http.MethodGet, "/api/v1/auth/me", nil, http.StatusUnauthorized, nil)
}

func TestStage4_WebPushSSRFProtection(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	ssrfEndpoints := []struct {
		name     string
		endpoint string
	}{
		{"Non-HTTPS scheme", "http://fcm.googleapis.com/fcm/send/123"},
		{"Custom port", "https://fcm.googleapis.com:8443/fcm/send/123"},
		{"Loopback localhost", "https://localhost/push"},
		{"Loopback IPv4", "https://127.0.0.1/push"},
		{"Cloud Metadata IP", "https://169.254.169.254/latest/meta-data/"},
		{"Private RFC1918 IP", "https://192.168.1.1/push"},
		{"Carrier-grade NAT IP", "https://100.64.0.1/push"},
	}

	for _, tc := range ssrfEndpoints {
		t.Run(tc.name, func(t *testing.T) {
			admin.DoJSON(t, http.MethodPost, "/api/v1/notifications/subscribe", map[string]any{
				"endpoint": tc.endpoint,
				"keys": map[string]string{
					"p256dh": "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM",
					"auth":   "tBHItJI5svbpez7KI4CCXg",
				},
			}, http.StatusBadRequest, nil)
		})
	}
}
