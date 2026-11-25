package e2e_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestCompetitionMembership_BanAndRoleOverrideEnforcedOnScopedRoutes(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)
	compA := admin.CompetitionSlug

	// Create a second competition in the same event.
	compB := CreateTestCompetition(t, admin, "comp-b-rbac", "Competition B RBAC").Slug
	// Switch admin back to compA.
	admin.CompetitionSlug = compA
	setActiveCompetitionSlug(compA)

	// Create two event-level participants.
	leader := CreateParticipant(t, admin, "leader.rbac@internal.example", "Leader RBAC")
	member := CreateParticipant(t, admin, "member.rbac@internal.example", "Member RBAC")
	promoted := CreateParticipant(t, admin, "promoted.rbac@internal.example", "Promoted Judge")

	// Leader creates a team in Comp A and member joins it.
	teamA := CreateTeamAs(t, leader, "Alpha RBAC")
	JoinTeamAs(t, member, teamA.JoinCode)

	// Enroll member in Comp A (participant) and ban them ONLY in Comp A.
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v2/e/%s/c/%s/users", admin.EventSlug, compA), map[string]any{
		"email": "member.rbac@internal.example",
		"name":  "Member RBAC",
		"role":  "participant",
	}, http.StatusCreated, nil)
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v2/e/%s/c/%s/members/%s/ban", admin.EventSlug, compA, member.UserID), nil, http.StatusOK, nil)

	// Competition-banned member must be rejected with 403 on Comp A team routes and cannot rejoin.
	member.CompetitionSlug = compA
	member.DoJSON(t, http.MethodGet, fmt.Sprintf("/api/v2/e/%s/c/%s/teams/me", member.EventSlug, compA), nil, http.StatusForbidden, nil)
	member.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v2/e/%s/c/%s/teams/join", member.EventSlug, compA), map[string]any{
		"join_code": teamA.JoinCode,
	}, http.StatusForbidden, nil)

	// The same user is NOT banned in Comp B and can still create a team there.
	member.CompetitionSlug = compB
	member.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v2/e/%s/c/%s/teams", member.EventSlug, compB), map[string]any{
		"name": "Comp B Team",
	}, http.StatusCreated, nil)

	// Assign competition-specific "judge" role to an event participant in Comp A.
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v2/e/%s/c/%s/users", admin.EventSlug, compA), map[string]any{
		"email": "promoted.rbac@internal.example",
		"name":  "Promoted Judge",
		"role":  "judge",
	}, http.StatusCreated, nil)

	// In Comp A, promoted user has judge permissions (allowed on judge panels, denied on participant-only team creation).
	promoted.CompetitionSlug = compA
	promoted.DoJSON(t, http.MethodGet, fmt.Sprintf("/api/v2/e/%s/c/%s/judging/judge/panels", promoted.EventSlug, compA), nil, http.StatusOK, nil)
	promoted.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v2/e/%s/c/%s/teams", promoted.EventSlug, compA), map[string]any{
		"name": "Should Fail In Comp A",
	}, http.StatusForbidden, nil)

	// In Comp B, promoted user remains an event participant (denied on judge panels, allowed to create a team).
	promoted.CompetitionSlug = compB
	promoted.DoJSON(t, http.MethodGet, fmt.Sprintf("/api/v2/e/%s/c/%s/judging/judge/panels", promoted.EventSlug, compB), nil, http.StatusForbidden, nil)
	promoted.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v2/e/%s/c/%s/teams", promoted.EventSlug, compB), map[string]any{
		"name": "Allowed In Comp B",
	}, http.StatusCreated, nil)
}
