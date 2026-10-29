package e2e_test

import (
	"fmt"
	"net/http"
	"testing"

	"devclubxnst.com/cadence/internal/modules/judging"
	"devclubxnst.com/cadence/internal/modules/users"
)

// Admin round controls: teams can be moved between rooms until the round starts, then the round
// can be ended, and only a round that is not active can be deleted.
func TestStage6_AdminRoundControls(t *testing.T) {
	ResetState(t)
	admin := LoginAsAdmin(t)

	var bulk users.BulkCreateResponse
	admin.DoJSON(t, http.MethodPost, "/api/v1/users/bulk", map[string]any{
		"users": []map[string]any{
			{"email": "ctl.judge1@cadence.local", "name": "Ctl Judge 1", "role": "judge"},
			{"email": "ctl.judge2@cadence.local", "name": "Ctl Judge 2", "role": "judge"},
		},
	}, http.StatusCreated, &bulk)
	if len(bulk.Users) != 2 {
		t.Fatalf("expected 2 judges created, got %d", len(bulk.Users))
	}
	judge1ID, judge2ID := bulk.Users[0].ID, bulk.Users[1].ID

	newTeam := func(name string) string {
		var team struct {
			ID string `json:"id"`
		}
		admin.DoJSON(t, http.MethodPost, "/api/v1/teams", map[string]any{"name": name}, http.StatusCreated, &team)
		return team.ID
	}
	teamA, teamB, teamC := newTeam("Ctl Team A"), newTeam("Ctl Team B"), newTeam("Ctl Team C")

	var round struct {
		RoundID string `json:"round_id"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/judging/rounds", map[string]any{
		"name":               "Control Round",
		"buffer_size":        1,
		"slot_duration_mins": 10,
		"criteria": []map[string]any{
			{"name": "Tech", "max_score": 10, "weight": 1.0, "order_index": 1},
		},
	}, http.StatusCreated, &round)

	newPanel := func(name string, judgeID string) string {
		var p struct {
			PanelID string `json:"panel_id"`
		}
		admin.DoJSON(t, http.MethodPost, "/api/v1/judging/panels", map[string]any{
			"round_id":  round.RoundID,
			"name":      name,
			"room":      "Room " + name,
			"judge_ids": []string{judgeID},
		}, http.StatusCreated, &p)
		return p.PanelID
	}
	panel1 := newPanel("One", judge1ID)
	panel2 := newPanel("Two", judge2ID)

	// A judge can only sit on one panel per round
	admin.DoJSON(t, http.MethodPost, "/api/v1/judging/panels", map[string]any{
		"round_id": round.RoundID, "name": "Dup", "room": "Dup", "judge_ids": []string{judge1ID},
	}, http.StatusConflict, nil)

	// Teams are distributed in stages: two now, the third later (append)
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/panels/%s/slots", panel1), map[string]any{
		"slots": []map[string]any{{"team_id": teamA, "sequence_order": 1}, {"team_id": teamB, "sequence_order": 2}},
	}, http.StatusOK, nil)
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/panels/%s/slots", panel2), map[string]any{
		"slots": []map[string]any{{"team_id": teamC, "sequence_order": 1}}, "append": true,
	}, http.StatusOK, nil)

	panelsOf := func() map[string][]judging.RoundPanelSlot {
		var list []judging.RoundPanelOverview
		admin.DoJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/judging/rounds/%s/panels", round.RoundID), nil, http.StatusOK, &list)
		out := map[string][]judging.RoundPanelSlot{}
		for _, p := range list {
			out[p.PanelID] = p.Slots
		}
		return out
	}

	// 1. Before the round starts, move Team B from room one to the front of room two
	before := panelsOf()
	if len(before[panel1]) != 2 || len(before[panel2]) != 1 {
		t.Fatalf("unexpected starting layout: %+v", before)
	}
	var teamBSlot string
	for _, s := range before[panel1] {
		if s.TeamID == teamB {
			teamBSlot = s.SlotID
		}
	}
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/slots/%s/move", teamBSlot), map[string]any{
		"target_panel_id": panel2, "target_sequence_order": 1,
	}, http.StatusOK, nil)

	after := panelsOf()
	if len(after[panel1]) != 1 || after[panel1][0].TeamID != teamA || after[panel1][0].SequenceOrder != 1 {
		t.Fatalf("room one should hold only Team A at #1 after the move, got %+v", after[panel1])
	}
	if len(after[panel2]) != 2 || after[panel2][0].TeamID != teamB || after[panel2][1].TeamID != teamC {
		t.Fatalf("room two should be [B, C] after the move, got %+v", after[panel2])
	}
	for _, s := range append(after[panel1], after[panel2]...) {
		if s.Status != "scheduled" {
			t.Fatalf("slots in a draft round must stay scheduled, got %q", s.Status)
		}
	}

	// 2. A draft round cannot be ended, but it can be deleted; make a throwaway to prove it
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/rounds/%s/end", round.RoundID), nil, http.StatusConflict, nil)
	var throwaway struct {
		RoundID string `json:"round_id"`
	}
	admin.DoJSON(t, http.MethodPost, "/api/v1/judging/rounds", map[string]any{
		"name": "Throwaway", "buffer_size": 1, "slot_duration_mins": 5,
		"criteria": []map[string]any{{"name": "C", "max_score": 10, "weight": 1.0, "order_index": 1}},
	}, http.StatusCreated, &throwaway)
	admin.DoJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/judging/rounds/%s", throwaway.RoundID), nil, http.StatusOK, nil)
	admin.DoJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/judging/rounds/%s", throwaway.RoundID), nil, http.StatusNotFound, nil)

	// 3. Once active, an active round cannot be deleted, and it must be ended first
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/rounds/%s/activate", round.RoundID), nil, http.StatusOK, nil)
	admin.DoJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/judging/rounds/%s", round.RoundID), nil, http.StatusConflict, nil)

	// 4. Ending an active round with no pitch running succeeds (the running-pitch guard needs a judge session)
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/rounds/%s/end", round.RoundID), nil, http.StatusOK, nil)
	var detail struct {
		Status string `json:"status"`
	}
	admin.DoJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/judging/rounds/%s", round.RoundID), nil, http.StatusOK, &detail)
	if detail.Status != "completed" {
		t.Fatalf("expected round status completed after ending, got %q", detail.Status)
	}
	// Ending twice is rejected; a finished round can be deleted
	admin.DoJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/judging/rounds/%s/end", round.RoundID), nil, http.StatusConflict, nil)
	admin.DoJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/judging/rounds/%s", round.RoundID), nil, http.StatusOK, nil)
	admin.DoJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/judging/rounds/%s", round.RoundID), nil, http.StatusNotFound, nil)
}
