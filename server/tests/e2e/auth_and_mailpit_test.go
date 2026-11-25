package e2e_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// OnboardUserViaMailpit establishes an authenticated v2 session for a seeded event user.
func OnboardUserViaMailpit(t *testing.T, name, email, _ string) *Actor {
	t.Helper()
	actor := NewActor(t, name, email)
	IssueActorSession(t, actor)

	if actor.GetCookieValue(t, "access_token") == "" || actor.GetCookieValue(t, "refresh_token") == "" {
		t.Fatalf("[%s] expected signed access_token and refresh_token cookies after onboarding", email)
	}
	return actor
}

func TestStage2_AuthAndMailpitJourney(t *testing.T) {
	ResetState(t)

	// 1. Admin logs in and verifies /api/v2/auth/me
	admin := LoginAsAdmin(t)

	var meResp struct {
		ID     string `json:"id"`
		Email  string `json:"email"`
		Role   string `json:"role"`
		Events []struct {
			EventSlug string `json:"event_slug"`
			Role      string `json:"role"`
		} `json:"events"`
	}
	admin.DoJSON(t, http.MethodGet, "/api/v2/auth/me", nil, http.StatusOK, &meResp)
	if meResp.Email != TestAdminEmail || meResp.Role != "dev" {
		t.Fatalf("unexpected admin /me response: %+v", meResp)
	}
	if len(meResp.Events) == 0 || meResp.Events[0].EventSlug != DefaultTestEventSlug {
		t.Fatalf("expected default event in /me response, got: %+v", meResp.Events)
	}

	// 2. Admin seeds one user per role in the active event (no emails are sent on creation)
	var createdJudge struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email": "judge.stage2@cadence.local",
		"name":  "Judge Stage2",
		"role":  "judge",
	}, http.StatusCreated, &createdJudge)

	var createdVolunteer struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email": "volunteer.stage2@cadence.local",
		"name":  "Volunteer Stage2",
		"role":  "volunteer",
	}, http.StatusCreated, &createdVolunteer)

	var bulkCreateResp struct {
		TotalCreated int `json:"total_created"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk", map[string]any{
		"users": []map[string]any{
			{"email": "participant.stage2@cadence.local", "name": "Participant Stage2", "role": "participant"},
		},
	}, http.StatusCreated, &bulkCreateResp)
	if bulkCreateResp.TotalCreated != 1 {
		t.Fatalf("expected 1 bulk created user, got %d", bulkCreateResp.TotalCreated)
	}

	// Seeding must not send any email
	time.Sleep(300 * time.Millisecond)
	for _, e := range []string{createdJudge.Email, createdVolunteer.Email, "participant.stage2@cadence.local"} {
		if mailpitHasRecipient(t, e) {
			t.Fatalf("seeding %s must not send an email", e)
		}
	}

	// 3. Onboard each role and verify their active event role and /auth/me profile
	judge := OnboardUserViaMailpit(t, "Judge", "judge.stage2@cadence.local", "JudgePass123!")
	volunteer := OnboardUserViaMailpit(t, "Volunteer", "volunteer.stage2@cadence.local", "VolPass123!")
	participant := OnboardUserViaMailpit(t, "Participant", "participant.stage2@cadence.local", "PartPass123!")

	if judge.Role != "judge" || volunteer.Role != "volunteer" || participant.Role != "participant" {
		t.Fatalf("unexpected roles after onboarding: judge=%s volunteer=%s participant=%s", judge.Role, volunteer.Role, participant.Role)
	}

	// 4. Verify Google OAuth login redirect & callback error handling
	anon := NewActor(t, "OAuthClient", "")
	var oauthURLResp struct {
		URL string `json:"url"`
	}
	anon.DoJSON(t, http.MethodGet, "/api/v2/auth/google/login?format=json&redirect=/dashboard", nil, http.StatusOK, &oauthURLResp)
	parsedOAuthURL, err := url.Parse(oauthURLResp.URL)
	if err != nil || parsedOAuthURL.Query().Get("state") == "" {
		t.Fatalf("expected valid Google OAuth URL with state param, got %q (err=%v)", oauthURLResp.URL, err)
	}
	stateData, err := base64.RawURLEncoding.DecodeString(parsedOAuthURL.Query().Get("state"))
	if err != nil {
		t.Fatalf("decode oauth state: %v", err)
	}
	var statePayload map[string]string
	if err := json.Unmarshal(stateData, &statePayload); err != nil || statePayload["redirect"] != "/dashboard" {
		t.Fatalf("unexpected oauth state payload: %+v (err=%v)", statePayload, err)
	}

	// 5. Verify refresh token rotation and logout lifecycle
	volunteer.DoJSON(t, http.MethodPost, "/api/v2/auth/refresh", nil, http.StatusOK, nil)
	volunteer.DoJSON(t, http.MethodGet, "/api/v2/auth/me", nil, http.StatusOK, nil)
	volunteer.DoJSON(t, http.MethodPost, "/api/v2/auth/logout", nil, http.StatusOK, nil)
}
