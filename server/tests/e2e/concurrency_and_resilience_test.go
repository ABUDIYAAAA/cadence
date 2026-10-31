package e2e_test

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"devclubxnst.com/cadence/internal/modules/tickets"
	"devclubxnst.com/cadence/internal/modules/users"
)

func TestStage5_ConcurrentTicketScanRaceCondition(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	// Create 1 participant and 10 volunteers to simulate 10 scanners firing at the exact same millisecond
	const numScanners = 10
	bulkUsers := []map[string]any{
		{
			"email":       "race.attendee@cadence.local",
			"name":        "Race Attendee",
			"role":        "participant",
			"send_invite": true,
		},
	}
	for i := 1; i <= numScanners; i++ {
		bulkUsers = append(bulkUsers, map[string]any{
			"email":       fmt.Sprintf("scanner.%d@cadence.local", i),
			"name":        fmt.Sprintf("Scanner Volunteer %d", i),
			"role":        "volunteer",
			"send_invite": true,
		})
	}

	var bulkResp users.BulkCreateResponse
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk", map[string]any{
		"users": bulkUsers,
	}, http.StatusCreated, &bulkResp)

	attendee := OnboardUserViaMailpit(t, "RaceAttendee", "race.attendee@cadence.local", "AttendeePass123!")
	scanners := make([]*Actor, numScanners)
	for i := 0; i < numScanners; i++ {
		email := fmt.Sprintf("scanner.%d@cadence.local", i+1)
		scanners[i] = OnboardUserViaMailpit(t, fmt.Sprintf("Scanner%d", i+1), email, "ScannerPass123!")
	}

	// Passes exist only once an admin dispatches them
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/tickets/users/%s/send", attendee.UserID), nil, http.StatusOK, nil)

	// Fetch attendee's ticket code
	var myTicket tickets.MyTicketResponse
	attendee.DoJSON(t, http.MethodGet, "/api/v1/tickets/me", nil, http.StatusOK, &myTicket)
	if myTicket.TicketCode == "" {
		t.Fatal("expected non-empty ticket code")
	}

	// Synchronize all 10 volunteer scanners on a start barrier
	startGate := make(chan struct{})
	var wg sync.WaitGroup
	var successCount int32
	var conflictCount int32
	var otherStatusCount int32

	for i := 0; i < numScanners; i++ {
		wg.Add(1)
		scanner := scanners[i]
		go func() {
			defer wg.Done()
			<-startGate

			resp, _ := scanner.DoRaw(t, http.MethodPost, "/api/v1/tickets/check-in", map[string]string{
				"ticket_code": myTicket.TicketCode,
			}, nil)

			switch resp.StatusCode {
			case http.StatusOK:
				atomic.AddInt32(&successCount, 1)
			case http.StatusConflict:
				atomic.AddInt32(&conflictCount, 1)
			default:
				atomic.AddInt32(&otherStatusCount, 1)
			}
		}()
	}

	// Release all 10 scanners simultaneously
	close(startGate)
	wg.Wait()

	if successCount != 1 || conflictCount != numScanners-1 || otherStatusCount != 0 {
		t.Fatalf(
			"Concurrent ticket check-in race condition failed: expected 1x 200 OK and %dx 409 Conflict, got 200=%d, 409=%d, other=%d",
			numScanners-1, successCount, conflictCount, otherStatusCount,
		)
	}
}

func TestStage5_BruteForceLoginGuardAndDeviceIsolation(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	admin.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email":       "bruteforce.target@cadence.local",
		"name":        "BruteForce Target",
		"role":        "participant",
		"send_invite": true,
	}, http.StatusCreated, nil)

	_ = OnboardUserViaMailpit(t, "BFTarget", "bruteforce.target@cadence.local", "CorrectPassword123!")

	// Attacker Device 1 makes 5 failed login attempts (DefaultMaxFailedLogins = 5)
	attackerDevice := NewActor(t, "AttackerDevice", "bruteforce.target@cadence.local")
	for attempt := 1; attempt <= 5; attempt++ {
		attackerDevice.DoJSON(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
			"email":    "bruteforce.target@cadence.local",
			"password": "WrongPassword999!",
		}, http.StatusUnauthorized, nil)
	}

	// 6th attempt from AttackerDevice — even with the CORRECT password — must be blocked with 429 Too Many Requests
	resp, _ := attackerDevice.DoRaw(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "bruteforce.target@cadence.local",
		"password": "CorrectPassword123!",
	}, nil)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests after 5 failed login attempts, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 429 login guard response")
	}

	// Legitimate User on a separate device behind the same Campus NAT IP must NOT be locked out!
	legitDevice := NewActor(t, "LegitDevice", "bruteforce.target@cadence.local")
	legitDevice.DoJSON(t, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "bruteforce.target@cadence.local",
		"password": "CorrectPassword123!",
	}, http.StatusOK, nil)
}
