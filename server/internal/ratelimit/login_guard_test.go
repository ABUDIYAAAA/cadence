package ratelimit

import (
	"context"
	"fmt"
	"testing"
)

func TestLoginGuard_MultiDimensionalBruteForceProtection(t *testing.T) {
	rdb := newTestRedis(t)
	guard := NewLoginGuard(rdb)
	ctx := context.Background()

	t.Run("single device repeated attempts against same email is locked out at DefaultMaxDeviceEmailLogins (5)", func(t *testing.T) {
		targetEmail := "victim-single-dev@cadence.local"
		deviceID := "single-device-1"
		peerIP := "198.51.100.22"

		for i := 1; i <= DefaultMaxDeviceEmailLogins; i++ {
			allowed, _, err := guard.CheckAllowed(ctx, deviceID, targetEmail, peerIP)
			if err != nil || !allowed {
				t.Fatalf("attempt %d expected allowed, got allowed=%v, err=%v", i, allowed, err)
			}
			if err := guard.RecordFailure(ctx, deviceID, targetEmail, peerIP); err != nil {
				t.Fatalf("record failure: %v", err)
			}
		}

		// 6th attempt from the same device must be blocked
		allowed, ttl, err := guard.CheckAllowed(ctx, deviceID, targetEmail, peerIP)
		if err != nil {
			t.Fatalf("check allowed: %v", err)
		}
		if allowed {
			t.Fatal("expected 6th attempt on same device to be blocked")
		}
		if ttl <= 0 {
			t.Fatalf("expected positive lockout TTL, got %v", ttl)
		}
	})

	t.Run("cookie-dropping attacker (fresh deviceID per attempt) reaches account lockout at DefaultMaxAccountAttempts (20)", func(t *testing.T) {
		targetEmail := "victim-account@cadence.local"
		attackerIP := "198.51.100.25"

		for i := 1; i <= DefaultMaxAccountAttempts; i++ {
			freshDeviceID := fmt.Sprintf("fresh-device-%d", i)
			allowed, _, err := guard.CheckAllowed(ctx, freshDeviceID, targetEmail, attackerIP)
			if err != nil {
				t.Fatalf("attempt %d CheckAllowed error: %v", i, err)
			}
			if !allowed {
				t.Fatalf("expected attempt %d to be allowed before reaching threshold", i)
			}
			if err := guard.RecordFailure(ctx, freshDeviceID, targetEmail, attackerIP); err != nil {
				t.Fatalf("attempt %d RecordFailure error: %v", i, err)
			}
		}

		// 21st attempt with yet another brand-new device cookie must be blocked!
		allowed, ttl, err := guard.CheckAllowed(ctx, "fresh-device-999", targetEmail, attackerIP)
		if err != nil {
			t.Fatalf("21st attempt CheckAllowed error: %v", err)
		}
		if allowed {
			t.Fatal("expected 21st failed login attempt to be blocked even when dropping _did cookie")
		}
		if ttl <= 0 {
			t.Fatalf("expected positive lockout TTL, got %v", ttl)
		}
	})

	t.Run("other accounts on the same campus IP remain unaffected when one account is locked out", func(t *testing.T) {
		campusIP := "198.51.100.25"
		allowed, _, err := guard.CheckAllowed(ctx, "other-user-device", "innocent@cadence.local", campusIP)
		if err != nil {
			t.Fatalf("CheckAllowed error: %v", err)
		}
		if !allowed {
			t.Fatal("expected innocent user on the same campus IP to be allowed")
		}
	})

	t.Run("password spraying across many accounts from one IP is locked out at DefaultMaxIPFailedLogins and cannot be reset by logging into attacker account", func(t *testing.T) {
		sprayIP := "198.51.100.99"
		const sprayLimit = 150
		guard := NewLoginGuard(rdb).WithMaxIPAttempts(sprayLimit)

		for i := 1; i <= sprayLimit; i++ {
			deviceID := fmt.Sprintf("spray-dev-%d", i)
			email := fmt.Sprintf("target-%d@cadence.local", i)
			if err := guard.RecordFailure(ctx, deviceID, email, sprayIP); err != nil {
				t.Fatalf("spray failure %d error: %v", i, err)
			}
		}

		// Attacker logs into their own account to try to clear the lockout
		if err := guard.ResetSuccess(ctx, "attacker-dev", "attacker@cadence.local"); err != nil {
			t.Fatalf("ResetSuccess error: %v", err)
		}

		// Next attempt from sprayIP against a brand new email must still be blocked by the per-IP counter
		allowed, _, err := guard.CheckAllowed(ctx, "spray-dev-next", "brandnew@cadence.local", sprayIP)
		if err != nil {
			t.Fatalf("CheckAllowed error: %v", err)
		}
		if allowed {
			t.Fatal("expected password spraying IP to remain locked out at DefaultMaxIPFailedLogins even after ResetSuccess")
		}
	})
}
