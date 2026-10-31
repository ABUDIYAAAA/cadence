package e2e_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"devclubxnst.com/cadence/internal/modules/teams"
	"devclubxnst.com/cadence/internal/modules/users"
)

func teamPath(id string, parts ...string) string {
	p := "/api/v2/teams/" + id
	for _, s := range parts {
		p += "/" + s
	}
	return p
}

func TestTeamsV2_CreateJoinLeaveAndLimits(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	p := make([]*Actor, 6)
	for i := range p {
		p[i] = CreateParticipant(t, admin, fmt.Sprintf("tv2.p%d@cadence.local", i+1), fmt.Sprintf("TV2 P%d", i+1))
	}

	team := CreateTeamAs(t, p[0], "Alpha")
	if team.JoinCode == "" || team.Leader == nil || team.Leader.ID != p[0].UserID || len(team.Members) != 1 {
		t.Fatalf("unexpected new team: %+v", team)
	}
	if team.EffectiveMaxSize != 4 {
		t.Fatalf("expected default max size 4 from env, got %d", team.EffectiveMaxSize)
	}

	// Staff search for people to enroll with no_team: only participants without a team come back.
	var free struct {
		TotalCount int `json:"total_count"`
	}
	admin.DoJSON(t, http.MethodGet, "/api/v1/users?role=participant&no_team=true", nil, http.StatusOK, &free)
	if free.TotalCount != 5 {
		t.Fatalf("expected 5 participants without a team, got %d", free.TotalCount)
	}
	admin.DoJSON(t, http.MethodGet, "/api/v1/users?role=participant&no_team=true&search=p2", nil, http.StatusOK, &free)
	if free.TotalCount != 1 {
		t.Fatalf("expected search to narrow to 1 participant, got %d", free.TotalCount)
	}

	// One team per participant
	p[0].DoJSON(t, http.MethodPost, "/api/v2/teams", map[string]any{"name": "Second"}, http.StatusConflict, nil)
	// Duplicate team names are rejected
	p[1].DoJSON(t, http.MethodPost, "/api/v2/teams", map[string]any{"name": "Alpha"}, http.StatusConflict, nil)

	// Invalid code, then fill the team to the default cap of 4
	p[1].DoJSON(t, http.MethodPost, "/api/v2/teams/join", map[string]any{"code": "ZZZZZZ"}, http.StatusNotFound, nil)
	for _, m := range p[1:4] {
		JoinTeamAs(t, m, team.JoinCode)
	}
	p[0].DoJSON(t, http.MethodPost, "/api/v2/teams/join", map[string]any{"code": team.JoinCode}, http.StatusConflict, nil)
	p[4].DoJSON(t, http.MethodPost, "/api/v2/teams/join", map[string]any{"code": team.JoinCode}, http.StatusConflict, nil)

	// A per-team override lets one more in
	admin.DoJSON(t, http.MethodPatch, teamPath(team.ID), map[string]any{"max_size": 5}, http.StatusOK, nil)
	JoinTeamAs(t, p[4], team.JoinCode)

	// Teammates see the code via my-team
	var mine teams.TeamDetailResponse
	p[2].DoJSON(t, http.MethodGet, "/api/v2/teams/my-team", nil, http.StatusOK, &mine)
	if mine.JoinCode != team.JoinCode || len(mine.Members) != 5 {
		t.Fatalf("unexpected my-team: %+v", mine)
	}

	// Only the leader can rename, regenerate the code or kick
	p[1].DoJSON(t, http.MethodPatch, "/api/v2/teams/my-team", map[string]any{"name": "Nope"}, http.StatusForbidden, nil)
	p[1].DoJSON(t, http.MethodPost, "/api/v2/teams/my-team/regenerate-code", nil, http.StatusForbidden, nil)
	p[1].DoJSON(t, http.MethodDelete, "/api/v2/teams/my-team/members/"+p[2].UserID, nil, http.StatusForbidden, nil)
	p[0].DoJSON(t, http.MethodDelete, "/api/v2/teams/my-team/members/"+p[0].UserID, nil, http.StatusBadRequest, nil)

	var renamed teams.TeamDetailResponse
	p[0].DoJSON(t, http.MethodPatch, "/api/v2/teams/my-team", map[string]any{"name": "Alpha Prime"}, http.StatusOK, &renamed)
	if renamed.Name != "Alpha Prime" {
		t.Fatalf("expected rename, got %q", renamed.Name)
	}

	var regen teams.TeamDetailResponse
	p[0].DoJSON(t, http.MethodPost, "/api/v2/teams/my-team/regenerate-code", nil, http.StatusOK, &regen)
	if regen.JoinCode == "" || regen.JoinCode == team.JoinCode {
		t.Fatalf("expected a new join code, got %q (old %q)", regen.JoinCode, team.JoinCode)
	}
	p[5].DoJSON(t, http.MethodPost, "/api/v2/teams/join", map[string]any{"code": team.JoinCode}, http.StatusNotFound, nil)

	// Kick a member, who is then free to start their own team
	p[0].DoJSON(t, http.MethodDelete, "/api/v2/teams/my-team/members/"+p[4].UserID, nil, http.StatusOK, nil)
	CreateTeamAs(t, p[4], "Epsilon")

	// Leader leaving hands leadership to the longest-standing member
	p[0].DoJSON(t, http.MethodPost, "/api/v2/teams/leave", nil, http.StatusOK, nil)
	var after teams.TeamDetailResponse
	p[1].DoJSON(t, http.MethodGet, "/api/v2/teams/my-team", nil, http.StatusOK, &after)
	if after.Leader == nil || after.Leader.ID != p[1].UserID || len(after.Members) != 3 {
		t.Fatalf("expected %s to lead a 3-member team, got %+v", p[1].UserID, after)
	}
	p[0].DoJSON(t, http.MethodGet, "/api/v2/teams/my-team", nil, http.StatusNotFound, nil)
	p[0].DoJSON(t, http.MethodPost, "/api/v2/teams/leave", nil, http.StatusConflict, nil)

	// A team whose last member leaves is removed
	p[4].DoJSON(t, http.MethodPost, "/api/v2/teams/leave", nil, http.StatusOK, nil)
	var list struct {
		TotalCount int `json:"total_count"`
	}
	admin.DoJSON(t, http.MethodGet, "/api/v2/teams", nil, http.StatusOK, &list)
	if list.TotalCount != 1 {
		t.Fatalf("expected only the Alpha team to remain, got %d teams", list.TotalCount)
	}
}

func TestTeamsV2_FreezeAndStaffOverrides(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	admin.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email": "tv2.vol@cadence.local", "name": "TV2 Volunteer", "role": "volunteer",
	}, http.StatusCreated, nil)
	volunteer := OnboardUserViaMailpit(t, "TV2Vol", "tv2.vol@cadence.local", "VolPass123!")

	lead := CreateParticipant(t, admin, "tv2.f1@cadence.local", "TV2 F1")
	mate := CreateParticipant(t, admin, "tv2.f2@cadence.local", "TV2 F2")
	late := CreateParticipant(t, admin, "tv2.f3@cadence.local", "TV2 F3")
	other := CreateParticipant(t, admin, "tv2.f4@cadence.local", "TV2 F4")

	team := CreateTeamAs(t, lead, "Freezers")
	JoinTeamAs(t, mate, team.JoinCode)

	// A volunteer freezes one team
	var frozen teams.TeamDetailResponse
	volunteer.DoJSON(t, http.MethodPost, teamPath(team.ID, "freeze"), nil, http.StatusOK, &frozen)
	if !frozen.IsFrozen || !frozen.EffectiveFrozen {
		t.Fatalf("expected team frozen, got %+v", frozen)
	}

	// Participants are locked out of join, leave, rename and kick
	late.DoJSON(t, http.MethodPost, "/api/v2/teams/join", map[string]any{"code": team.JoinCode}, http.StatusConflict, nil)
	mate.DoJSON(t, http.MethodPost, "/api/v2/teams/leave", nil, http.StatusConflict, nil)
	lead.DoJSON(t, http.MethodPatch, "/api/v2/teams/my-team", map[string]any{"name": "Renamed"}, http.StatusConflict, nil)
	lead.DoJSON(t, http.MethodDelete, "/api/v2/teams/my-team/members/"+mate.UserID, nil, http.StatusConflict, nil)

	// Staff can still edit a frozen team
	volunteer.DoJSON(t, http.MethodPatch, teamPath(team.ID), map[string]any{"name": "Staff Renamed"}, http.StatusOK, nil)
	volunteer.DoJSON(t, http.MethodPost, teamPath(team.ID, "members"), map[string]any{"user_id": late.UserID}, http.StatusOK, nil)
	volunteer.DoJSON(t, http.MethodDelete, teamPath(team.ID, "members", late.UserID), nil, http.StatusOK, nil)
	// Staff cannot move someone who already has a team
	volunteer.DoJSON(t, http.MethodPost, teamPath(team.ID, "members"), map[string]any{"user_id": mate.UserID}, http.StatusConflict, nil)

	volunteer.DoJSON(t, http.MethodPost, teamPath(team.ID, "unfreeze"), nil, http.StatusOK, nil)
	JoinTeamAs(t, late, team.JoinCode)

	// Freeze all blocks new teams and changes on every team
	var settings teams.SettingsResponse
	volunteer.DoJSON(t, http.MethodPost, "/api/v2/teams/freeze-all", nil, http.StatusOK, &settings)
	if !settings.FrozenAll {
		t.Fatalf("expected frozen_all, got %+v", settings)
	}
	other.DoJSON(t, http.MethodPost, "/api/v2/teams", map[string]any{"name": "Too Late"}, http.StatusConflict, nil)
	late.DoJSON(t, http.MethodPost, "/api/v2/teams/leave", nil, http.StatusConflict, nil)
	volunteer.DoJSON(t, http.MethodPost, "/api/v2/teams/unfreeze-all", nil, http.StatusOK, &settings)
	if settings.FrozenAll {
		t.Fatalf("expected frozen_all cleared, got %+v", settings)
	}
	late.DoJSON(t, http.MethodPost, "/api/v2/teams/leave", nil, http.StatusOK, nil)

	// Only admins change the global max size
	volunteer.DoJSON(t, http.MethodPatch, "/api/v2/teams/settings", map[string]any{"max_team_size": 6}, http.StatusForbidden, nil)
	admin.DoJSON(t, http.MethodPatch, "/api/v2/teams/settings", map[string]any{"max_team_size": 6}, http.StatusOK, &settings)
	if settings.MaxTeamSize != 6 || settings.MaxTeamSizeSource != "database" {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestTeamsV2_ConcurrentJoinsRespectMaxSize(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	admin.DoJSON(t, http.MethodPatch, "/api/v2/teams/settings", map[string]any{"max_team_size": 2}, http.StatusOK, nil)

	lead := CreateParticipant(t, admin, "tv2.c0@cadence.local", "TV2 C0")
	const joiners = 5
	actors := make([]*Actor, joiners)
	for i := range actors {
		actors[i] = CreateParticipant(t, admin, fmt.Sprintf("tv2.c%d@cadence.local", i+1), fmt.Sprintf("TV2 C%d", i+1))
	}
	team := CreateTeamAs(t, lead, "Race")

	statuses := make([]int, joiners)
	var wg sync.WaitGroup
	for i, a := range actors {
		wg.Add(1)
		go func(i int, a *Actor) {
			defer wg.Done()
			resp, _ := a.DoRaw(t, http.MethodPost, "/api/v2/teams/join", map[string]any{"code": team.JoinCode}, nil)
			statuses[i] = resp.StatusCode
		}(i, a)
	}
	wg.Wait()

	ok := 0
	for _, s := range statuses {
		if s == http.StatusOK {
			ok++
		} else if s != http.StatusConflict {
			t.Fatalf("unexpected join status %d (all: %v)", s, statuses)
		}
	}
	if ok != 1 {
		t.Fatalf("expected exactly 1 successful join into a 2-person team, got %d (%v)", ok, statuses)
	}
}

func TestTeamsV2_BulkImportIgnoresTeamFields(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	var bulk users.BulkCreateResponse
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk", map[string]any{
		"users": []map[string]any{
			{"email": "tv2.bulk@cadence.local", "name": "Bulk One", "role": "participant", "team_name": "Ghost", "is_leader": true},
		},
	}, http.StatusCreated, &bulk)
	if bulk.TotalCreated != 1 || bulk.Users[0].TeamID != nil {
		t.Fatalf("bulk import must not attach teams: %+v", bulk)
	}
	var list struct {
		TotalCount int `json:"total_count"`
	}
	admin.DoJSON(t, http.MethodGet, "/api/v2/teams", nil, http.StatusOK, &list)
	if list.TotalCount != 0 {
		t.Fatalf("bulk import must not create teams, got %d", list.TotalCount)
	}
}

// A participant who registered with a wrong email contacts a volunteer, who adds them with the right one.
// The new participant then sets their own password via "forgot password"; no invite email is ever sent.
func TestVolunteerAddsParticipantsOnly(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	admin.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email": "add.vol@cadence.local", "name": "Add Volunteer", "role": "volunteer",
	}, http.StatusCreated, nil)
	volunteer := OnboardUserViaMailpit(t, "AddVol", "add.vol@cadence.local", "VolPass123!")

	// Role is forced to participant when omitted; anything else is refused.
	var created users.UserDetailResponse
	volunteer.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email": "add.fixed@cadence.local", "name": "Fixed Email",
	}, http.StatusCreated, &created)
	if created.Role != "participant" || created.IsPasswordConfigured {
		t.Fatalf("expected a password-less participant, got %+v", created)
	}
	volunteer.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email": "add.judge@cadence.local", "name": "Sneaky", "role": "judge",
	}, http.StatusForbidden, nil)
	volunteer.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email": "add.admin@cadence.local", "name": "Sneakier", "role": "admin",
	}, http.StatusForbidden, nil)

	// Everything else about user management stays admin-only.
	volunteer.DoJSON(t, http.MethodPost, "/api/v1/users/bulk", map[string]any{
		"users": []map[string]any{{"email": "add.bulk@cadence.local", "name": "Bulk"}},
	}, http.StatusForbidden, nil)
	volunteer.DoJSON(t, http.MethodPatch, "/api/v1/users/"+created.ID, map[string]any{"role": "admin"}, http.StatusForbidden, nil)
	volunteer.DoJSON(t, http.MethodPost, "/api/v1/users/"+created.ID+"/trigger-reset-password", nil, http.StatusForbidden, nil)

	// The invite endpoints are gone.
	volunteer.DoJSON(t, http.MethodPost, "/api/v1/users/"+created.ID+"/send-invite", nil, http.StatusNotFound, nil)
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk-send-invite", map[string]any{"all_uninvited": true}, http.StatusMethodNotAllowed, nil)

	// No email until the participant asks for one, then they can sign in.
	time.Sleep(300 * time.Millisecond)
	if mailpitHasRecipient(t, "add.fixed@cadence.local") {
		t.Fatal("adding a participant must not send an email")
	}
	p := OnboardUserViaMailpit(t, "FixedEmail", "add.fixed@cadence.local", "PartPass123!")
	if p.UserID != created.ID {
		t.Fatalf("signed in as %s, expected %s", p.UserID, created.ID)
	}
}

// Forgot-password sends email, so a mailbox can only be asked about a handful of times per window,
// and the answer does not depend on whether the account exists.
func TestForgotPasswordIsThrottledPerEmail(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)
	admin.DoJSON(t, http.MethodPost, "/api/v1/users", map[string]any{
		"email": "throttle.me@cadence.local", "name": "Throttle Me", "role": "participant",
	}, http.StatusCreated, nil)

	for _, email := range []string{"throttle.me@cadence.local", "nobody.here@cadence.local"} {
		// Fresh devices each time, so only the per-email limit can trigger.
		for i := 1; i <= 5; i++ {
			NewActor(t, "T", email).DoJSON(t, http.MethodPost, "/api/v1/auth/forgot-password",
				map[string]string{"email": email}, http.StatusOK, nil)
		}
		resp, _ := NewActor(t, "T", email).DoRaw(t, http.MethodPost, "/api/v1/auth/forgot-password",
			map[string]string{"email": email}, nil)
		if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") == "" {
			t.Fatalf("%s: expected 429 with Retry-After on the 6th request, got %d", email, resp.StatusCode)
		}
	}

	// Another mailbox is unaffected.
	NewActor(t, "T", "other@cadence.local").DoJSON(t, http.MethodPost, "/api/v1/auth/forgot-password",
		map[string]string{"email": "other@cadence.local"}, http.StatusOK, nil)
}
