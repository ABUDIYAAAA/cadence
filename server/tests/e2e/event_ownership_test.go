package e2e_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestEventOwnershipTransfer_OwnerAndDevAllowed(t *testing.T) {
	ResetState(t)
	dev := LoginAsAdmin(t) // seeded TestAdminEmail has platform role "dev"

	// Create two regular users in the default event.
	ownerA := CreateParticipant(t, dev, "owner.a@internal.example", "Owner A")
	ownerB := CreateParticipant(t, dev, "owner.b@internal.example", "Owner B")
	ownerC := CreateParticipant(t, dev, "owner.c@internal.example", "Owner C")

	// Dev creates a fresh event owned by Owner A.
	const slug = "transfer-test-event"
	dev.DoJSON(t, http.MethodPost, "/api/v2/events", map[string]any{
		"slug":        slug,
		"name":        "Transfer Test Event",
		"owner_email": "owner.a@internal.example",
	}, http.StatusCreated, nil)

	// 1. Platform dev (who is NOT Owner A) can transfer ownership from Owner A to Owner B.
	dev.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v2/e/%s/transfer-ownership", slug), map[string]any{
		"new_owner_user_id": ownerB.UserID,
	}, http.StatusOK, nil)

	// Owner A is now demoted to admin and cannot transfer ownership anymore (403).
	ownerA.EventSlug = slug
	ownerA.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v2/e/%s/transfer-ownership", slug), map[string]any{
		"new_owner_user_id": ownerC.UserID,
	}, http.StatusForbidden, nil)

	// 2. Owner B (current event owner) can transfer ownership to Owner C.
	ownerB.EventSlug = slug
	ownerB.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v2/e/%s/transfer-ownership", slug), map[string]any{
		"new_owner_user_id": ownerC.UserID,
	}, http.StatusOK, nil)

	// Verify Owner C is now the owner via /me.
	ownerC.EventSlug = slug
	var me struct {
		Role string `json:"role"`
	}
	ownerC.DoJSON(t, http.MethodGet, fmt.Sprintf("/api/v2/e/%s/me", slug), nil, http.StatusOK, &me)
	if me.Role != "owner" {
		t.Fatalf("expected Owner C role to be 'owner', got %q", me.Role)
	}
}
