package judging

import (
	"context"
	"errors"
	"testing"
)

func TestTeamCriteria_ImportScopingAndRoster(t *testing.T) {
	pool, _ := newTestJudgingPool(t, 17)
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
	// Re-import is allowed while unscored and replaces the previous rows.
	if err := repo.ImportTeamCriteria(ctx, roundID, ImportTeamCriteriaRequest{
		Weight: 20,
		Rows: []TeamCriterionRow{
			{TeamID: teamA, CriterionName: "Hardware", MaxScore: 15},
			{TeamID: teamB, CriterionName: "Software", MaxScore: 30},
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
	if err := repo.ConfigureSlots(ctx, panelID, []SlotAssignment{{TeamID: teamC, SequenceOrder: 1}}, false); !errors.Is(err, ErrTeamNotInRound) {
		t.Fatalf("expected ErrTeamNotInRound for off-roster team, got %v", err)
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
	_ = slotB
}
