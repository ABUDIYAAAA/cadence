package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/modules/auth"
	"devclubxnst.com/cadence/internal/modules/competitions"
	"devclubxnst.com/cadence/internal/modules/teams"
	"devclubxnst.com/cadence/internal/ratelimit"
	"devclubxnst.com/cadence/pkg/utils"
)

// Actor represents a distinct user/device session with its own isolated HTTP CookieJar.
type Actor struct {
	Name            string
	Email           string
	Role            string
	UserID          string
	EventSlug       string
	CompetitionSlug string
	Client          *http.Client
	CookieJar       http.CookieJar
	BaseURL         string
}

// NewActor creates a new Actor with an isolated cookie jar so signed cookies (_did, access_token, refresh_token)
// are maintained independently per simulated role/device.
func NewActor(t *testing.T, name, email string) *Actor {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to create cookie jar for actor %s: %v", name, err)
	}

	return &Actor{
		Name:            name,
		Email:           email,
		EventSlug:       DefaultTestEventSlug,
		CompetitionSlug: getActiveCompetitionSlug(),
		CookieJar:       jar,
		BaseURL:         testServer.URL,
		Client: &http.Client{
			Jar:     jar,
			Timeout: 15 * time.Second,
		},
	}
}

func matchesPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/") || strings.HasPrefix(path, prefix+"?")
}

// rewriteToV2TenantPath translates legacy /api/v1/* and unscoped /api/v2/* paths onto the v2
// /api/v2/e/{eventSlug} and /api/v2/e/{eventSlug}/c/{competitionSlug} route hierarchy,
// matching client/lib/tenant-scope.ts.
func (a *Actor) rewriteToV2TenantPath(path string) string {
	if !strings.HasPrefix(path, "/api/") {
		return path
	}
	rel := path
	if strings.HasPrefix(rel, "/api/v1/") {
		rel = "/" + strings.TrimPrefix(rel, "/api/v1/")
	} else if strings.HasPrefix(rel, "/api/v2/") {
		rel = "/" + strings.TrimPrefix(rel, "/api/v2/")
	}
	if strings.HasPrefix(rel, "/e/") || matchesPrefix(rel, "/auth") || matchesPrefix(rel, "/events") {
		return "/api/v2" + rel
	}

	eventSlug := a.EventSlug
	if eventSlug == "" {
		eventSlug = DefaultTestEventSlug
	}
	compSlug := a.CompetitionSlug
	if compSlug == "" {
		compSlug = getActiveCompetitionSlug()
	}

	for _, prefix := range []string{"/teams", "/judging"} {
		if matchesPrefix(rel, prefix) {
			if compSlug != "" {
				return fmt.Sprintf("/api/v2/e/%s/c/%s%s", eventSlug, compSlug, rel)
			}
			return "/api/v2" + rel
		}
	}

	for _, prefix := range []string{"/tickets", "/notifications", "/audit-logs", "/competitions", "/users", "/members"} {
		if matchesPrefix(rel, prefix) {
			return fmt.Sprintf("/api/v2/e/%s%s", eventSlug, rel)
		}
	}

	return "/api/v2" + rel
}

// DoRaw executes an HTTP request and returns the response and raw body bytes.
func (a *Actor) DoRaw(
	t *testing.T,
	method, path string,
	reqBody any,
	headers map[string]string,
) (*http.Response, []byte) {
	t.Helper()

	resolvedPath := a.rewriteToV2TenantPath(path)

	var bodyReader io.Reader
	var rawReqBytes []byte
	if reqBody != nil {
		var err error
		rawReqBytes, err = json.Marshal(reqBody)
		if err != nil {
			t.Fatalf("[Actor %s] failed to marshal request body for %s %s: %v", a.Name, method, resolvedPath, err)
		}
		bodyReader = bytes.NewReader(rawReqBytes)
	}

	fullURL := a.BaseURL + resolvedPath
	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		t.Fatalf("[Actor %s] failed to create request %s %s: %v", a.Name, method, resolvedPath, err)
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", fmt.Sprintf("Cadence-E2E-Actor/%s", a.Name))

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		t.Fatalf("[Actor %s] HTTP request %s %s failed: %v", a.Name, method, resolvedPath, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("[Actor %s] failed reading response body from %s %s: %v", a.Name, method, resolvedPath, err)
	}

	return resp, respBytes
}

// DoJSON executes a JSON request, asserts the exact HTTP status code, and unmarshals the response into out (if non-nil).
func (a *Actor) DoJSON(
	t *testing.T,
	method, path string,
	reqBody any,
	expectedStatus int,
	out any,
) []byte {
	t.Helper()

	resp, respBytes := a.DoRaw(t, method, path, reqBody, nil)
	if resp.StatusCode != expectedStatus {
		reqJSON, _ := json.Marshal(reqBody)
		t.Fatalf(
			"[Actor %s (%s)] %s %s (resolved %s): expected status %d, got %d\n  Request Body : %s\n  Response Body: %s",
			a.Name, a.Email, method, path, a.rewriteToV2TenantPath(path), expectedStatus, resp.StatusCode, string(reqJSON), string(respBytes),
		)
	}

	if out != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, out); err != nil {
			t.Fatalf(
				"[Actor %s (%s)] %s %s: failed to unmarshal response JSON into %T: %v\n  Raw Body: %s",
				a.Name, a.Email, method, path, out, err, string(respBytes),
			)
		}
	}

	return respBytes
}

// GetCookieValue returns the raw (signed) cookie value for the given cookie name from the Actor's CookieJar.
func (a *Actor) GetCookieValue(t *testing.T, cookieName string) string {
	t.Helper()
	u, err := url.Parse(a.BaseURL)
	if err != nil {
		t.Fatalf("invalid base URL %s: %v", a.BaseURL, err)
	}
	for _, c := range a.CookieJar.Cookies(u) {
		if c.Name == cookieName {
			return c.Value
		}
	}
	return ""
}

// SetCookieValue sets or overwrites a cookie in the Actor's CookieJar.
func (a *Actor) SetCookieValue(t *testing.T, cookieName, value string) {
	t.Helper()
	u, err := url.Parse(a.BaseURL)
	if err != nil {
		t.Fatalf("invalid base URL %s: %v", a.BaseURL, err)
	}
	a.CookieJar.SetCookies(u, []*http.Cookie{
		{
			Name:  cookieName,
			Value: value,
			Path:  "/",
		},
	})
}

// IssueActorSession mints a real device-bound JWT access_token + refresh_token session in PostgreSQL
// and stores the signed cookies (_did, access_token, refresh_token) in the Actor's CookieJar.
func IssueActorSession(t *testing.T, actor *Actor) *Actor {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ensure actor has a verified signed _did cookie
	actor.DoJSON(t, http.MethodGet, "/health", nil, http.StatusOK, nil)
	var deviceID string
	u, _ := url.Parse(actor.BaseURL)
	req := &http.Request{Header: http.Header{}}
	for _, c := range actor.CookieJar.Cookies(u) {
		req.AddCookie(c)
	}
	deviceID, err := testApp.CookieManager.GetSignedCookie(req, ratelimit.DeviceIDCookieName)
	if err != nil || deviceID == "" {
		deviceID, _ = utils.GenerateRandomToken(16)
	}

	var userID, platformRole string
	var tokenVersion int
	err = testApp.DB.QueryRow(ctx, `
		SELECT id, role::text, token_version
		FROM users
		WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL
	`, actor.Email).Scan(&userID, &platformRole, &tokenVersion)
	if err != nil {
		t.Fatalf("[IssueActorSession] failed to find user %s: %v", actor.Email, err)
	}

	var eventRole string
	_ = testApp.DB.QueryRow(ctx, `
		SELECT ue.role::text
		FROM user_events ue
		JOIN events e ON e.id = ue.event_id
		WHERE ue.user_id = $1 AND LOWER(e.slug) = LOWER($2)
	`, userID, actor.EventSlug).Scan(&eventRole)

	accessToken, err := utils.GenerateAccessToken(
		userID,
		actor.Email,
		platformRole,
		tokenVersion,
		testApp.Config.JWTAccessSecret,
		testApp.Config.JWTAccessExpiry,
	)
	if err != nil {
		t.Fatalf("[IssueActorSession] generate access token: %v", err)
	}

	rawRefreshToken, err := utils.GenerateRandomToken(32)
	if err != nil {
		t.Fatalf("[IssueActorSession] generate refresh token: %v", err)
	}

	ua := fmt.Sprintf("Cadence-E2E-Actor/%s", actor.Name)
	ip := "127.0.0.1"
	authRepo := auth.NewRepository(testApp.DB)
	if err := authRepo.CreateRefreshToken(ctx, &auth.RefreshToken{
		UserID:    userID,
		DeviceID:  deviceID,
		TokenHash: utils.HashToken(rawRefreshToken),
		UserAgent: &ua,
		IPAddress: &ip,
		ExpiresAt: time.Now().Add(testApp.Config.JWTRefreshExpiry),
	}); err != nil {
		t.Fatalf("[IssueActorSession] persist refresh token: %v", err)
	}

	cookieRec := httptest.NewRecorder()
	testApp.CookieManager.SetSignedCookie(cookieRec, ratelimit.DeviceIDCookieName, deviceID, 365*24*time.Hour)
	testApp.CookieManager.SetSignedCookie(cookieRec, "access_token", accessToken, testApp.Config.AccessCookieExpiry)
	testApp.CookieManager.SetSignedCookie(cookieRec, "refresh_token", rawRefreshToken, testApp.Config.RefreshCookieExpiry)
	actor.CookieJar.SetCookies(u, cookieRec.Result().Cookies())

	actor.UserID = userID
	if eventRole != "" && platformRole != "dev" {
		actor.Role = eventRole
	} else {
		actor.Role = platformRole
	}
	return actor
}

// CreateTestCompetition explicitly creates a competition inside the active event via the v2 API
// and sets it as the active competition for the current test (no silent 'main' fallback).
func CreateTestCompetition(t *testing.T, admin *Actor, slug, name string) competitions.Competition {
	t.Helper()
	var comp competitions.Competition
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v2/e/%s/competitions", admin.EventSlug), map[string]any{
		"slug": slug,
		"name": name,
	}, http.StatusCreated, &comp)
	setActiveCompetitionSlug(comp.Slug)
	admin.CompetitionSlug = comp.Slug
	return comp
}

// LoginAsAdmin authenticates the seeded root administrator and creates a dedicated competition for the test.
func LoginAsAdmin(t *testing.T) *Actor {
	t.Helper()
	admin := NewActor(t, "Admin", TestAdminEmail)
	IssueActorSession(t, admin)

	seq := compCounter.Add(1)
	compSlug := fmt.Sprintf("comp-%d", seq)
	CreateTestCompetition(t, admin, compSlug, fmt.Sprintf("Competition %d", seq))
	return admin
}

// WaitForCondition polls checkFn at interval until it returns true or timeout expires.
func WaitForCondition(
	t *testing.T,
	timeout, interval time.Duration,
	description string,
	checkFn func() (bool, error),
	diagFn func() string,
) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		ok, err := checkFn()
		if err == nil && ok {
			return
		}
		lastErr = err
		time.Sleep(interval)
	}

	diag := ""
	if diagFn != nil {
		diag = "\nDiagnostics:\n" + diagFn()
	}
	t.Fatalf("Timed out after %s waiting for condition: %s (lastErr: %v)%s", timeout, description, lastErr, diag)
}

// CreateParticipant creates a participant through the event users API and issues their session cookies.
func CreateParticipant(t *testing.T, admin *Actor, email, name string) *Actor {
	t.Helper()
	admin.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email": email, "name": name, "role": "participant",
	}, http.StatusCreated, nil)
	return OnboardUserViaMailpit(t, name, email, "PartPass123!")
}

// CreateTeamAs creates a team through the v2 competition-scoped API as the given participant (who becomes leader).
func CreateTeamAs(t *testing.T, p *Actor, name string) teams.TeamDetailResponse {
	t.Helper()
	var team teams.TeamDetailResponse
	p.DoJSON(t, http.MethodPost, "/api/v2/teams", map[string]any{"name": name}, http.StatusCreated, &team)
	return team
}

// JoinTeamAs joins a team through the v2 competition-scoped API using its join code.
func JoinTeamAs(t *testing.T, p *Actor, code string) teams.TeamDetailResponse {
	t.Helper()
	var team teams.TeamDetailResponse
	p.DoJSON(t, http.MethodPost, "/api/v2/teams/join", map[string]any{"code": code}, http.StatusOK, &team)
	return team
}
