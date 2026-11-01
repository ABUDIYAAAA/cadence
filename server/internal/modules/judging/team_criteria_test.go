package judging

import (
	"context"
	"errors"
	"testing"
)

func TestTeamCriteria_ImportScopingAndRoster(t *testing.T) {
	pool, _ := newTestJudgingPool(t, 19)
	repo := NewRepository(pool)
	svc := NewService(repo, nil)
	ctx := context.Background()

	var judgeID, teamA, teamB, teamC string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, is_active, is_password_configured)
		VALUES ('judge.tc@cadence.local', 'Judge TC', 'judge', true, true) RETURNING id`).Scan(&judgeID); err != nil {
		t.Fatalf("seed judge: %v", err)
	}
	for code, dst := range map[string]*string{"TEAMA001": &teamA, "TEAMB002": &teamB, "TEAMC003": &teamC} {
		if err := pool.QueryRow(ctx, `INSERT INTO teams (name, join_code) VALUES ($1, $1) RETURNING id`, code).Scan(dst); err != nil {
			t.Fatalf("seed team: %v", err)
		}
	}

	roundID, err := repo.CreateRound(ctx, CreateRoundRequest{
		Name: "R", BufferSize: 2, SlotDurationMins: 10,
		Criteria: []CreateCriteriaRequest{{Name: "Shared", MinScore: 0, MaxScore: 10, Weight: 50, OrderIndex: 1}},
	})
	if err != nil {
		t.Fatalf("create round: %v", err)
	}

	if err := repo.ImportTeamCriteria(ctx, roundID, ImportTeamCriteriaRequest{
		Weight: 20,
		Rows: []TeamCriterionRow{
			{TeamID: teamA, CriterionName: "Hardware", MaxScore: 15},
			{TeamID: teamA, CriterionName: "Dup", MaxScore: 5},
		},
	}); !errors.Is(err, ErrDuplicateTeamRow) {
		t.Fatalf("expected ErrDuplicateTeamRow, got %v", err)
	}

	if err := repo.ImportTeamCriteria(ctx, roundID, ImportTeamCriteriaRequest{
		Weight: 20,
		Rows: []TeamCriterionRow{
			{TeamID: teamA, CriterionName: "Hardware", MaxScore: 15},
			{TeamID: teamB, CriterionName: "Software", MaxScore: 30},
		},
	}); err != nil {
		t.Fatalf("import: %v", err)
	}
	awarded := 24.0
	over := 31.0
	if err := repo.ImportTeamCriteria(ctx, roundID, ImportTeamCriteriaRequest{
		Weight: 20,
		Rows:   []TeamCriterionRow{{TeamID: teamB, CriterionName: "Software", MaxScore: 30, Score: &over}},
	}); !errors.Is(err, ErrAwardedScoreRange) {
		t.Fatalf("expected ErrAwardedScoreRange, got %v", err)
	}
	// Re-import is allowed while unscored and replaces the previous rows.
	if err := repo.ImportTeamCriteria(ctx, roundID, ImportTeamCriteriaRequest{
		Weight: 20,
		Rows: []TeamCriterionRow{
			{TeamID: teamA, CriterionName: "Hardware", MaxScore: 15},
			{TeamID: teamB, CriterionName: "Software", MaxScore: 30, Score: &awarded},
		},
	}); err != nil {
		t.Fatalf("re-import: %v", err)
	}

	if shared, _ := repo.GetRoundCriteria(ctx, roundID); len(shared) != 1 {
		t.Fatalf("round-wide criteria should exclude team criteria, got %d", len(shared))
	}
	if roster, _ := repo.GetRoundTeams(ctx, roundID); len(roster) != 2 {
		t.Fatalf("expected roster of 2, got %d", len(roster))
	}
	if err := repo.ActivateRound(ctx, roundID); err != nil {
		t.Fatalf("activate: %v", err)
	}

	panelID, err := repo.CreatePanel(ctx, CreatePanelRequest{RoundID: roundID, Name: "P", Room: "R1", JudgeIDs: []string{judgeID}})
	if err != nil {
		t.Fatalf("panel: %v", err)
	}
	if err := repo.ConfigureSlots(ctx, panelID, []SlotAssignment{
		{TeamID: teamA, SequenceOrder: 1}, {TeamID: teamB, SequenceOrder: 2},
	}, false); err != nil {
		t.Fatalf("configure: %v", err)
	}

	slotOf := func(team string) string {
		var id string
		if err := pool.QueryRow(ctx, `SELECT id FROM judging_slots WHERE panel_id = $1 AND team_id = $2`, panelID, team).Scan(&id); err != nil {
			t.Fatalf("slot: %v", err)
		}
		return id
	}
	slotA, slotB := slotOf(teamA), slotOf(teamB)
	if err := repo.StartSlot(ctx, slotA); err != nil {
		t.Fatalf("start A: %v", err)
	}

	d, err := repo.GetJudgeSlotDetails(ctx, slotA, judgeID)
	if err != nil {
		t.Fatalf("details: %v", err)
	}
	if len(d.CriteriaScores) != 2 {
		t.Fatalf("judge on team A should see shared + own criterion, got %d", len(d.CriteriaScores))
	}
	var ownID, otherID string
	for _, c := range d.CriteriaScores {
		if c.CriterionName == "Hardware" {
			ownID = c.CriterionID
			if c.MaxScore != 15 {
				t.Fatalf("team A max want 15 got %d", c.MaxScore)
			}
		}
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM judging_criteria WHERE round_id = $1 AND team_id = $2`, roundID, teamB).Scan(&otherID); err != nil {
		t.Fatalf("other crit: %v", err)
	}

	if err := svc.SubmitScores(ctx, slotA, judgeID, SubmitScoresRequest{Scores: []CriterionScoreInput{{CriterionID: otherID, Score: 5}}}); !errors.Is(err, ErrCriterionNotInRound) {
		t.Fatalf("scoring another team's criterion must be rejected, got %v", err)
	}
	if err := svc.SubmitScores(ctx, slotA, judgeID, SubmitScoresRequest{Scores: []CriterionScoreInput{{CriterionID: ownID, Score: 16}}}); !errors.Is(err, ErrScoreOutOfRange) {
		t.Fatalf("score above the team's max must be rejected, got %v", err)
	}

	// Team B's criterion is pre-awarded: judges neither see nor score it.
	db, err := repo.GetJudgeSlotDetails(ctx, slotB, judgeID)
	if err != nil {
		t.Fatalf("details B: %v", err)
	}
	if len(db.CriteriaScores) != 1 {
		t.Fatalf("judge on team B should only see the shared criterion, got %d", len(db.CriteriaScores))
	}
	if err := svc.SubmitScores(ctx, slotB, judgeID, SubmitScoresRequest{Scores: []CriterionScoreInput{{CriterionID: otherID, Score: 5}}}); err == nil {
		t.Fatal("scoring a pre-awarded criterion must be rejected")
	}

	// Its awarded score counts in the team's leaderboard total: 24/30 * weight 20 = 16.
	lb, err := repo.GetAdminLeaderboard(ctx, roundID)
	if err != nil {
		t.Fatalf("leaderboard: %v", err)
	}
	for _, e := range lb.Leaderboard {
		if e.TeamID == teamB && (e.TotalWeightedScore < 15.99 || e.TotalWeightedScore > 16.01) {
			t.Fatalf("team B total want 16, got %v", e.TotalWeightedScore)
		}
	}
}

func TestTeamCriteria_OffRosterTeamWithManualMarks(t *testing.T) {
	pool, _ := newTestJudgingPool(t, 19)
	repo := NewRepository(pool)
	ctx := context.Background()

	var judgeID, imported, extra string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, is_active, is_password_configured)
		VALUES ('judge.or@cadence.local', 'Judge OR', 'judge', true, true) RETURNING id`).Scan(&judgeID); err != nil {
		t.Fatal(err)
	}
	for code, dst := range map[string]*string{"IMPORT01": &imported, "EXTRA002": &extra} {
		if err := pool.QueryRow(ctx, `INSERT INTO teams (name, join_code) VALUES ($1, $1) RETURNING id`, code).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	roundID, err := repo.CreateRound(ctx, CreateRoundRequest{
		Name: "R", BufferSize: 2, SlotDurationMins: 10,
		Criteria: []CreateCriteriaRequest{{Name: "Shared", MinScore: 0, MaxScore: 10, Weight: 50, OrderIndex: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ImportTeamCriteria(ctx, roundID, ImportTeamCriteriaRequest{
		Weight: 20, Rows: []TeamCriterionRow{{TeamID: imported, CriterionName: "Hardware", MaxScore: 10}},
	}); err != nil {
		t.Fatal(err)
	}
	panelID, err := repo.CreatePanel(ctx, CreatePanelRequest{RoundID: roundID, Name: "P", Room: "R1", JudgeIDs: []string{judgeID}})
	if err != nil {
		t.Fatal(err)
	}

	// A team that wasn't in the CSV can be scheduled and joins the roster.
	if err := repo.ConfigureSlots(ctx, panelID, []SlotAssignment{{TeamID: imported, SequenceOrder: 1}, {TeamID: extra, SequenceOrder: 2}}, false); err != nil {
		t.Fatalf("off-roster team should be schedulable: %v", err)
	}
	if roster, _ := repo.GetRoundTeams(ctx, roundID); len(roster) != 2 {
		t.Fatalf("roster should now hold both teams, got %d", len(roster))
	}

	// Marks must stay within the max.
	if err := repo.SetTeamCriterion(ctx, roundID, extra, SetTeamCriterionRequest{CriterionName: "Hardware", MaxScore: 10, Weight: 20, Score: 11}); !errors.Is(err, ErrAwardedScoreRange) {
		t.Fatalf("expected ErrAwardedScoreRange, got %v", err)
	}
	if err := repo.SetTeamCriterion(ctx, roundID, extra, SetTeamCriterionRequest{CriterionName: "Hardware", MaxScore: 10, Weight: 20, Score: 5}); err != nil {
		t.Fatalf("set marks: %v", err)
	}
	// Editing again updates in place.
	if err := repo.SetTeamCriterion(ctx, roundID, extra, SetTeamCriterionRequest{CriterionName: "Hardware", MaxScore: 10, Weight: 20, Score: 10}); err != nil {
		t.Fatalf("edit marks: %v", err)
	}

	lb, err := repo.GetAdminLeaderboard(ctx, roundID)
	if err != nil {
		t.Fatalf("leaderboard: %v", err)
	}
	var found bool
	for _, e := range lb.Leaderboard {
		if e.TeamID == extra {
			found = true
			if e.TotalWeightedScore < 19.99 || e.TotalWeightedScore > 20.01 {
				t.Fatalf("manual marks should add 20 to the total, got %v", e.TotalWeightedScore)
			}
			if e.CriteriaAverages["Hardware"] != 10 {
				t.Fatalf("leaderboard should list the custom criterion, got %v", e.CriteriaAverages)
			}
		}
	}
	if !found {
		t.Fatal("off-roster team missing from leaderboard")
	}

	if err := repo.DeleteTeamCriterion(ctx, roundID, extra); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
