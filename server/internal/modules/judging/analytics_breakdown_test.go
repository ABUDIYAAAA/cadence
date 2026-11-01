package judging

import (
	"context"
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 0.001 }

func TestAnalytics_CustomCriterionAddsToTotalsAndIsTransparent(t *testing.T) {
	pool, _ := newTestJudgingPool(t, 20)
	repo := NewRepository(pool)
	svc := NewService(repo, nil)
	ctx := context.Background()

	mkJudge := func(email string) string {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO users (email, name, role, is_active, is_password_configured)
			VALUES ($1, $1, 'judge', true, true) RETURNING id`, email).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	j1, j2 := mkJudge("an1@cadence.local"), mkJudge("an2@cadence.local")
	var team string
	if err := pool.QueryRow(ctx, `INSERT INTO teams (name, join_code) VALUES ('ANTEAM01', 'ANTEAM01') RETURNING id`).Scan(&team); err != nil {
		t.Fatal(err)
	}

	roundID, err := repo.CreateRound(ctx, CreateRoundRequest{
		Name: "R", BufferSize: 2, SlotDurationMins: 10,
		Criteria: []CreateCriteriaRequest{
			{Name: "Idea", MinScore: 0, MaxScore: 10, Weight: 30, OrderIndex: 1},
			{Name: "Demo", MinScore: 0, MaxScore: 10, Weight: 20, OrderIndex: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	award := 8.0
	if err := repo.ImportTeamCriteria(ctx, roundID, ImportTeamCriteriaRequest{
		Weight: 50, Rows: []TeamCriterionRow{{TeamID: team, CriterionName: "Hardware", MaxScore: 10, Score: &award}},
	}); err != nil {
		t.Fatal(err)
	}
	panelID, err := repo.CreatePanel(ctx, CreatePanelRequest{RoundID: roundID, Name: "P", Room: "R1", JudgeIDs: []string{j1, j2}})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ConfigureSlots(ctx, panelID, []SlotAssignment{{TeamID: team, SequenceOrder: 1}}, false); err != nil {
		t.Fatal(err)
	}
	if err := repo.ActivateRound(ctx, roundID); err != nil {
		t.Fatal(err)
	}
	var slotID string
	_ = pool.QueryRow(ctx, `SELECT id FROM judging_slots WHERE panel_id = $1`, panelID).Scan(&slotID)
	if err := repo.StartSlot(ctx, slotID); err != nil {
		t.Fatal(err)
	}
	crit, err := repo.GetRoundCriteria(ctx, roundID)
	if err != nil || len(crit) != 2 {
		t.Fatalf("criteria: %v (%d)", err, len(crit))
	}
	idea, demo := crit[0].ID, crit[1].ID

	// Only judge 1 scores and locks; judge 2 is still pending.
	if err := svc.SubmitScores(ctx, slotID, j1, SubmitScoresRequest{Scores: []CriterionScoreInput{
		{CriterionID: idea, Score: 8}, {CriterionID: demo, Score: 6},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.LockJudgeScorecard(ctx, slotID, j1); err != nil {
		t.Fatal(err)
	}

	// Leaderboard: total = judged (8/10*30 + 6/10*20 = 36) + custom (8/10*50 = 40) = 76 of 100.
	lb, err := repo.GetAdminLeaderboard(ctx, roundID)
	if err != nil || len(lb.Leaderboard) != 1 {
		t.Fatalf("leaderboard: %v", err)
	}
	e := lb.Leaderboard[0]
	if !approx(e.JudgedScore, 36) || !approx(e.CustomScore, 40) || !approx(e.TotalWeightedScore, 76) || !approx(e.MaxPossibleScore, 100) {
		t.Fatalf("judged=%v custom=%v total=%v max=%v, want 36/40/76/100", e.JudgedScore, e.CustomScore, e.TotalWeightedScore, e.MaxPossibleScore)
	}
	if e.CustomCriterion != "Hardware" || e.CustomAwarded == nil || *e.CustomAwarded != 8 {
		t.Fatalf("custom detail missing: %+v", e)
	}

	// Analytics criteria show real numbers, and the custom criterion appears by its own name.
	an, err := repo.GetRoundAnalytics(ctx, roundID)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]CriterionAnalytics{}
	for _, c := range an.Criteria {
		byName[c.Name] = c
	}
	if c := byName["Idea"]; c.Scores != 1 || !approx(c.AvgScore, 8) {
		t.Fatalf("Idea analytics = %+v, want 1 score averaging 8", c)
	}
	if c := byName["Hardware"]; !c.Custom || c.Scores != 1 || !approx(c.AvgScore, 8) || !approx(c.Weight, 50) {
		t.Fatalf("Hardware analytics = %+v, want custom, 1 team, avg 8, weight 50", c)
	}
	if _, bad := byName["Team-specific criterion"]; bad {
		t.Fatal("custom criterion should be listed by its real name")
	}

	// Score sheet: judge 1's two marks, judge 2's two pending blanks, and the custom mark.
	sheet, err := repo.GetScoreSheet(ctx, roundID)
	if err != nil {
		t.Fatal(err)
	}
	var scored, pending, custom int
	for _, r := range sheet.Rows {
		switch {
		case r.Kind == "custom" && r.Score != nil && *r.Score == 8:
			custom++
		case r.Kind == "judged" && r.Score != nil && r.Locked && r.JudgeID == j1:
			scored++
		case r.Kind == "judged" && r.Score == nil && r.JudgeID == j2:
			pending++
		}
	}
	if scored != 2 || pending != 2 || custom != 1 || len(sheet.Rows) != 5 {
		t.Fatalf("score sheet scored=%d pending=%d custom=%d total=%d, want 2/2/1/5", scored, pending, custom, len(sheet.Rows))
	}
}
