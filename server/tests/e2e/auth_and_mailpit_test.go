package e2e_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"devclubxnst.com/cadence/pkg/audit"
)

// OnboardUserViaMailpit signs a seeded user in for the first time the way real users do: no password
// exists yet, so they request a reset link with "forgot password", read it from Mailpit, choose a password
// via /api/v1/auth/reset-password and then log in. It returns an Actor with signed session cookies.
func OnboardUserViaMailpit(t *testing.T, name, email, password string) *Actor {
	t.Helper()

	actor := NewActor(t, name, email)
	actor.DoJSON(t, http.MethodPost, "/api/v1/auth/forgot-password", map[string]string{"email": email}, http.StatusOK, nil)

	msg := WaitForEmail(t, email, "Cadence - Password Reset Request")
	resetToken := ExtractTokenFromEmailHTML(t, msg.HTML, "reset-password")

	actor.DoJSON(t, http.MethodPost, "/api/v1/auth/reset-password", map[string]string{
		"token":            resetToken,
		"password":         password,
		"confirm_password": password,
	}, http.StatusOK, nil)

	// Single-use token protection: reusing the same token must return 410 Gone
	NewActor(t, name+"-replay", email).DoJSON(t, http.MethodPost, "/api/v1/auth/reset-password", map[string]string{
		"token":            resetToken,
		"password":         "AnotherPassword123!",
		"confirm_password": "AnotherPassword123!",
	}, http.StatusGone, nil)

	var loginResp struct {
		User struct {
			ID                   string `json:"id"`
			Email                string `json:"email"`
			Role                 string `json:"role"`
			IsPasswordConfigured bool   `json:"is_password_configured"`
		} `json:"user"`
	}
	actor.DoJSON(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, http.StatusOK, &loginResp)

	if !loginResp.User.IsPasswordConfigured {
		t.Fatalf("[%s] expected is_password_configured=true after reset-password", email)
	}
	if actor.GetCookieValue(t, "access_token") == "" || actor.GetCookieValue(t, "refresh_token") == "" {
		t.Fatalf("[%s] expected signed access_token and refresh_token cookies after login", email)
	}

	actor.UserID = loginResp.User.ID
	actor.Role = loginResp.User.Role
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

	// 2. Admin seeds one user per role (no emails are sent; people set their own password later)
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
	time.Sleep(500 * time.Millisecond)
	for _, e := range []string{createdJudge.Email, createdVolunteer.Email, "participant.stage2@cadence.local"} {
		if mailpitHasRecipient(t, e) {
			t.Fatalf("seeding %s must not send an email", e)
		}
	}

	// Logging in before a password exists must fail with 403 Forbidden
	preSetupVol := NewActor(t, "VolunteerPreSetup", createdVolunteer.Email)
	preSetupVol.DoJSON(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    createdVolunteer.Email,
		"password": "AnyPassword123!",
	}, http.StatusForbidden, nil)

	// 3. Everyone sets a password through "forgot password"
	judge := OnboardUserViaMailpit(t, "Judge", "judge.stage2@cadence.local", "JudgePass123!")
	volunteer := OnboardUserViaMailpit(t, "Volunteer", "volunteer.stage2@cadence.local", "VolPass123!")
	participant := OnboardUserViaMailpit(t, "Participant", "participant.stage2@cadence.local", "PartPass123!")

	if judge.Role != "judge" || volunteer.Role != "volunteer" || participant.Role != "participant" {
		t.Fatalf("unexpected roles after onboarding: judge=%s volunteer=%s participant=%s", judge.Role, volunteer.Role, participant.Role)
	}

	// 4. Self-service Forgot Password -> Mailpit reset link -> Reset Password -> Re-login
	ClearMailpitFor(t, volunteer.Email) // drop the onboarding reset mail so we only see the new link
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
	ClearMailpitFor(t, participant.Email)
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
		"audit logs for auth.password_forgot and auth.password_reset persisted",
		func() (bool, error) {
			var auditResp struct {
				TotalCount int64 `json:"total_count"`
				Data       []struct {
					Action string `json:"action"`
				} `json:"data"`
			}
			admin.DoJSON(t, http.MethodGet, "/api/v1/audit-logs?limit=100", nil, http.StatusOK, &auditResp)
			lastActions = nil
			hasForgot := false
			hasReset := false
			for _, row := range auditResp.Data {
				lastActions = append(lastActions, row.Action)
				if row.Action == audit.ActionAuthPasswordForgot {
					hasForgot = true
				}
				if row.Action == audit.ActionAuthPasswordReset {
					hasReset = true
				}
			}
			return hasForgot && hasReset, nil
		},
		func() string {
			return "Persisted audit actions: " + strings.Join(lastActions, ", ")
		},
	)
}
