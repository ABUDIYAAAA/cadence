package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
	"time"
)

// Actor represents a distinct user/device session with its own isolated HTTP CookieJar.
type Actor struct {
	Name      string
	Email     string
	Role      string
	UserID    string
	Client    *http.Client
	CookieJar http.CookieJar
	BaseURL   string
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
		Name:      name,
		Email:     email,
		CookieJar: jar,
		BaseURL:   testServer.URL,
		Client: &http.Client{
			Jar:     jar,
			Timeout: 15 * time.Second,
		},
	}
}

// DoRaw executes an HTTP request and returns the response and raw body bytes.
func (a *Actor) DoRaw(
	t *testing.T,
	method, path string,
	reqBody any,
	headers map[string]string,
) (*http.Response, []byte) {
	t.Helper()

	var bodyReader io.Reader
	var rawReqBytes []byte
	if reqBody != nil {
		var err error
		rawReqBytes, err = json.Marshal(reqBody)
		if err != nil {
			t.Fatalf("[Actor %s] failed to marshal request body for %s %s: %v", a.Name, method, path, err)
		}
		bodyReader = bytes.NewReader(rawReqBytes)
	}

	fullURL := a.BaseURL + path
	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		t.Fatalf("[Actor %s] failed to create request %s %s: %v", a.Name, method, path, err)
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
		t.Fatalf("[Actor %s] HTTP request %s %s failed: %v", a.Name, method, path, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("[Actor %s] failed reading response body from %s %s: %v", a.Name, method, path, err)
	}

	return resp, respBytes
}

// DoJSON executes a JSON request, asserts the exact HTTP status code, and unmarshals the response into out (if non-nil).
// On unexpected status code, prints full request/response diagnostics.
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
			"[Actor %s (%s)] %s %s: expected status %d, got %d\n  Request Body : %s\n  Response Body: %s",
			a.Name, a.Email, method, path, expectedStatus, resp.StatusCode, string(reqJSON), string(respBytes),
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

// LoginAsAdmin logs in using the seeded root administrator credentials and returns the authenticated Admin actor.
func LoginAsAdmin(t *testing.T) *Actor {
	t.Helper()
	admin := NewActor(t, "Admin", TestAdminEmail)

	var loginResp struct {
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"user"`
		Message string `json:"message"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    TestAdminEmail,
		"password": TestAdminPassword,
	}, http.StatusOK, &loginResp)

	admin.UserID = loginResp.User.ID
	admin.Role = loginResp.User.Role
	return admin
}

// WaitForCondition polls checkFn at interval until it returns true or timeout expires.
// On timeout, invokes diagFn() to include diagnostic state in the test failure output.
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
