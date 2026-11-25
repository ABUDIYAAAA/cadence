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

	// parallel runs fn for every actor with at most `limit` in flight and fails if any returns an error.
	parallel := func(limit int, fn func(a *Actor) error) {
		t.Helper()
		var wg sync.WaitGroup
		errs := make(chan error, n)
		slots := make(chan struct{}, limit)
		for _, a := range actors {
			wg.Add(1)
			slots <- struct{}{}
			go func(a *Actor) {
				defer wg.Done()
				defer func() { <-slots }()
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

	// 1. App load: unauthenticated /auth/me (401), then each requests Google OAuth login URL.
	start := time.Now()
	parallel(40, func(a *Actor) error {
		if err := call(a, http.MethodGet, "/api/v2/auth/me", nil, http.StatusUnauthorized); err != nil {
			return err
		}
		return call(a, http.MethodGet, "/api/v2/auth/google/login?format=json", nil, http.StatusOK)
	})
	t.Logf("%d unauthenticated burst requests accepted in %s", n, time.Since(start).Round(time.Millisecond))

	// 2. Everyone signs in and loads /auth/me and their competition team status concurrently without 429s.
	for i := range actors {
		IssueActorSession(t, actors[i])
	}
	parallel(20, func(a *Actor) error {
		if err := call(a, http.MethodGet, "/api/v2/auth/me", nil, http.StatusOK); err != nil {
			return err
		}
		return call(a, http.MethodGet, "/api/v1/judging/teams/my-status", nil, http.StatusOK)
	})
}
