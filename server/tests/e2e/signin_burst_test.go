package e2e_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

// Event day: everyone is seeded without a password, then ~1,000 people behind one campus IP open the app and
// request a reset link at once. This runs the same shape at 200 users from a single IP (127.0.0.1): every
// unauthenticated call must succeed (no 429s from the shared IP limits) and every reset email must arrive.
func TestSignInBurst_SeededUsersSetPasswordsTogether(t *testing.T) {
	const n = 200
	ResetState(t)
	admin := LoginAsAdmin(t)

	seed := make([]map[string]any, n)
	for i := range seed {
		seed[i] = map[string]any{"email": fmt.Sprintf("burst%03d@cadence.local", i), "name": fmt.Sprintf("Burst %03d", i), "role": "participant"}
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk", map[string]any{"users": seed}, http.StatusCreated, nil)

	actors := make([]*Actor, n)
	for i := range actors {
		actors[i] = NewActor(t, fmt.Sprintf("Burst%03d", i), fmt.Sprintf("burst%03d@cadence.local", i))
	}

	// parallel runs fn for every actor at once and fails if any returns an error.
	parallel := func(fn func(a *Actor) error) {
		t.Helper()
		var wg sync.WaitGroup
		errs := make(chan error, n)
		for _, a := range actors {
			wg.Add(1)
			go func(a *Actor) {
				defer wg.Done()
				if err := fn(a); err != nil {
					errs <- fmt.Errorf("%s: %w", a.Email, err)
				}
			}(a)
		}
		wg.Wait()
		close(errs)
		if len(errs) > 0 {
			t.Fatalf("%d of %d users failed, first: %v", len(errs), n, <-errs)
		}
	}
	call := func(a *Actor, method, path string, body any, want int) error {
		resp, raw := a.DoRaw(t, method, path, body, nil)
		if resp.StatusCode != want {
			return fmt.Errorf("%s %s: expected %d, got %d: %s", method, path, want, resp.StatusCode, raw)
		}
		return nil
	}

	// 1. App load: unauthenticated /auth/me (401), then each asks for a reset link.
	start := time.Now()
	parallel(func(a *Actor) error {
		if err := call(a, http.MethodGet, "/api/v1/auth/me", nil, http.StatusUnauthorized); err != nil {
			return err
		}
		return call(a, http.MethodPost, "/api/v1/auth/forgot-password", map[string]string{"email": a.Email}, http.StatusOK)
	})
	t.Logf("%d forgot-password requests accepted in %s", n, time.Since(start).Round(time.Millisecond))

	// 2. Wait for the mail workers to deliver every reset email.
	deadline := time.Now().Add(60 * time.Second)
	var list *MailpitMessagesListResponse
	for {
		var err error
		if list, err = fetchMailpitMessages(); err != nil {
			t.Fatalf("fetch mailpit: %v", err)
		}
		if len(list.Messages) >= n {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d reset emails arrived within 60s", len(list.Messages), n)
		}
		time.Sleep(300 * time.Millisecond)
	}
	tokens := map[string]string{}
	for _, m := range list.Messages {
		detail, err := fetchMailpitMessageDetail(m.ID)
		if err != nil || len(m.To) == 0 {
			t.Fatalf("fetch message %s: %v", m.ID, err)
		}
		tokens[m.To[0].Address] = ExtractTokenFromEmailHTML(t, detail.HTML, "reset-password")
	}
	t.Logf("all %d reset emails delivered %s after the first request", n, time.Since(start).Round(time.Millisecond))

	// 3. Everyone sets a password and signs in at the same time, then loads /auth/me.
	parallel(func(a *Actor) error {
		pw := "BurstPass123!"
		if err := call(a, http.MethodPost, "/api/v1/auth/reset-password",
			map[string]string{"token": tokens[a.Email], "password": pw, "confirm_password": pw}, http.StatusOK); err != nil {
			return err
		}
		if err := call(a, http.MethodPost, "/api/v1/auth/login", map[string]string{"email": a.Email, "password": pw}, http.StatusOK); err != nil {
			return err
		}
		return call(a, http.MethodGet, "/api/v1/auth/me", nil, http.StatusOK)
	})
}
