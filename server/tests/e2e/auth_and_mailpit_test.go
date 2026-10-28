package e2e_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"devclubxnst.com/cadence/pkg/audit"
)

// OnboardUserViaMailpit waits for the user's setup-password invite email in Mailpit,
// extracts the real setup token from the HTML body, configures their password via /api/v1/auth/setup-password,
// and returns an authenticated Actor with signed session cookies.
func OnboardUserViaMailpit(t *testing.T, name, email, password string) *Actor {
	t.Helper()

	msg := WaitForEmail(t, email, "Welcome to Cadence - Set Up Your Password")
	setupToken := ExtractTokenFromEmailHTML(t, msg.HTML, "setup-password")

	actor := NewActor(t, name, email)

	var setupResp struct {
		User struct {
			ID                   string `json:"id"`
			Email                string `json:"email"`
			Name                 string `json:"name"`
			Role                 string `json:"role"`
			IsPasswordConfigured bool   `json:"is_password_configured"`
		} `json:"user"`
		Message string `json:"message"`
	}

	actor.DoJSON(t, http.MethodPost, "/api/v1/auth/setup-password", map[string]string{
		"token":            setupToken,
		"password":         password,
		"confirm_password": password,
	}, http.StatusOK, &setupResp)

	if !setupResp.User.IsPasswordConfigured {
		t.Fatalf("[%s] expected is_password_configured=true after setup-password", email)
	}
	if actor.GetCookieValue(t, "access_token") == "" || actor.GetCookieValue(t, "refresh_token") == "" {
		t.Fatalf("[%s] expected signed access_token and refresh_token cookies after setup-password", email)
	}

	actor.UserID = setupResp.User.ID
	actor.Role = setupResp.User.Role

	// Verify single-use setup token protection: reusing the same token must return 410 Gone
	replayActor := NewActor(t, name+"-replay", email)
	replayActor.DoJSON(t, http.MethodPost, "/api/v1/auth/setup-password", map[string]string{
		"token":            setupToken,
		"password":         "AnotherPassword123!",
		"confirm_password": "AnotherPassword123!",
	}, http.StatusGone, nil)

	return actor
}

func TestStage2_AuthAndMailpitJourney(t *testing.T) {
	ResetState(t)

	// 1. Admin logs in and verifies /api/v1/auth/me
	admin := LoginAsAdmin(t)

	var meResp struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	admin.DoJSON(t, http.MethodGet, "/api/v1/auth/me", nil, http.StatusOK, &meResp)
	if meResp.Email != TestAdminEmail || meResp.Role != "admin" {
		t.Fatalf("unexpected admin /me response: %+v", meResp)
	}

	// 2. Admin creates one user per role (Judge, Volunteer, Participant) using different invite flows
	// 2a. Judge created via POST /api/v1/users with send_invite=true
	var createdJudge struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email":       "judge.stage2@cadence.local",
		"name":        "Judge Stage2",
		"role":        "judge",
		"send_invite": true,
	}, http.StatusCreated, &createdJudge)

	// 2b. Volunteer created with send_invite=false, then invited via POST /api/v1/users/{id}/send-invite
	var createdVolunteer struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email":       "volunteer.stage2@cadence.local",
		"name":        "Volunteer Stage2",
		"role":        "volunteer",
		"send_invite": false,
	}, http.StatusCreated, &createdVolunteer)

	// Attempting to log in before password setup must fail with 403 Forbidden
	preSetupVol := NewActor(t, "VolunteerPreSetup", createdVolunteer.Email)
	preSetupVol.DoJSON(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    createdVolunteer.Email,
		"password": "AnyPassword123!",
	}, http.StatusForbidden, nil)

	// Trigger single setup invite for Volunteer
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/users/%s/send-invite", createdVolunteer.ID), nil, http.StatusOK, nil)

	// 2c. Participant created via bulk import with send_invite=false, then invited via /api/v1/users/bulk-send-invite
	var bulkCreateResp struct {
		TotalCreated int `json:"total_created"`
		Users        []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"users"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk", map[string]any{
		"users": []map[string]any{
			{
				"email":       "participant.stage2@cadence.local",
				"name":        "Participant Stage2",
				"role":        "participant",
				"team_name":   "Stage2 Explorers",
				"is_leader":   true,
				"send_invite": false,
			},
		},
	}, http.StatusCreated, &bulkCreateResp)
	if bulkCreateResp.TotalCreated != 1 {
		t.Fatalf("expected 1 bulk created user, got %d", bulkCreateResp.TotalCreated)
	}

	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk-send-invite", map[string]any{
		"all_uninvited": true,
	}, http.StatusOK, nil)

	// 3. Onboard all 3 roles by polling Mailpit, extracting real HTML setup tokens, and setting passwords
	judge := OnboardUserViaMailpit(t, "Judge", "judge.stage2@cadence.local", "JudgePass123!")
	volunteer := OnboardUserViaMailpit(t, "Volunteer", "volunteer.stage2@cadence.local", "VolPass123!")
	participant := OnboardUserViaMailpit(t, "Participant", "participant.stage2@cadence.local", "PartPass123!")

	if judge.Role != "judge" || volunteer.Role != "volunteer" || participant.Role != "participant" {
		t.Fatalf("unexpected roles after onboarding: judge=%s volunteer=%s participant=%s", judge.Role, volunteer.Role, participant.Role)
	}

	// Sending an invite again to an already-configured user must return 400 Bad Request
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/users/%s/send-invite", judge.UserID), nil, http.StatusBadRequest, nil)

	// 4. Self-service Forgot Password -> Mailpit reset link -> Reset Password -> Re-login
	anon := NewActor(t, "ForgotPasswordClient", volunteer.Email)
	anon.DoJSON(t, http.MethodPost, "/api/v1/auth/forgot-password", map[string]string{
		"email": volunteer.Email,
	}, http.StatusOK, nil)

	resetMsg := WaitForEmail(t, volunteer.Email, "Cadence - Password Reset Request")
	resetToken := ExtractTokenFromEmailHTML(t, resetMsg.HTML, "reset-password")

	anon.DoJSON(t, http.MethodPost, "/api/v1/auth/reset-password", map[string]string{
		"token":            resetToken,
		"password":         "NewVolunteerPass456!",
		"confirm_password": "NewVolunteerPass456!",
	}, http.StatusOK, nil)

	// Reusing the reset token must return 410 Gone
	anon.DoJSON(t, http.MethodPost, "/api/v1/auth/reset-password", map[string]string{
		"token":            resetToken,
		"password":         "AnotherPass789!",
		"confirm_password": "AnotherPass789!",
	}, http.StatusGone, nil)

	// Old password must fail (401), new password must succeed (200)
	volRelogin := NewActor(t, "VolunteerRelogin", volunteer.Email)
	volRelogin.DoJSON(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    volunteer.Email,
		"password": "VolPass123!",
	}, http.StatusUnauthorized, nil)
	volRelogin.DoJSON(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    volunteer.Email,
		"password": "NewVolunteerPass456!",
	}, http.StatusOK, nil)

	// 5. Admin-triggered Password Reset for Participant -> Mailpit -> Reset Password -> Re-login
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/users/%s/trigger-reset-password", participant.UserID), nil, http.StatusOK, nil)

	partResetMsg := WaitForEmail(t, participant.Email, "Cadence - Password Reset Request")
	partResetToken := ExtractTokenFromEmailHTML(t, partResetMsg.HTML, "reset-password")

	partResetActor := NewActor(t, "ParticipantReset", participant.Email)
	partResetActor.DoJSON(t, http.MethodPost, "/api/v1/auth/reset-password", map[string]string{
		"token":            partResetToken,
		"password":         "NewPartPass999!",
		"confirm_password": "NewPartPass999!",
	}, http.StatusOK, nil)

	partResetActor.DoJSON(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    participant.Email,
		"password": "NewPartPass999!",
	}, http.StatusOK, nil)

	// 6. Verify asynchronous Audit Logs were persisted via RabbitMQ consumer
	var lastActions []string
	WaitForCondition(
		t,
		10*time.Second,
		150*time.Millisecond,
		"audit logs for auth.password_setup and auth.password_reset persisted",
		func() (bool, error) {
			var auditResp struct {
				TotalCount int64 `json:"total_count"`
				Data       []struct {
					Action string `json:"action"`
				} `json:"data"`
			}
			admin.DoJSON(t, http.MethodGet, "/api/v1/audit-logs?limit=100", nil, http.StatusOK, &auditResp)
			lastActions = nil
			hasSetup := false
			hasReset := false
			for _, row := range auditResp.Data {
				lastActions = append(lastActions, row.Action)
				if row.Action == audit.ActionAuthPasswordSetup {
					hasSetup = true
				}
				if row.Action == audit.ActionAuthPasswordReset {
					hasReset = true
				}
			}
			return hasSetup && hasReset, nil
		},
		func() string {
			return "Persisted audit actions: " + strings.Join(lastActions, ", ")
		},
	)
}
