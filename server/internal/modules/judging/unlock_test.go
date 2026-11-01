package judging

import (
	"context"
	"errors"
	"testing"
)

func TestUnlock_JudgeAndAdminFlows(t *testing.T) {
	pool, _ := newTestJudgingPool(t, 20)
	repo := NewRepository(pool)
	svc := NewService(repo, nil)
	ctx := context.Background()

	mk := func(email, role string) string {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO users (email, name, role, is_active, is_password_configured)
			VALUES ($1, $1, $2, true, true) RETURNING id`, email, role).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	judge, other, admin := mk("ul1@cadence.local", "judge"), mk("ul2@cadence.local", "judge"), mk("uladmin@cadence.local", "admin")
	teams := make([]string, 2)
	for i, code := range []string{"ULTEAM01", "ULTEAM02"} {
		if err := pool.QueryRow(ctx, `INSERT INTO teams (name, join_code) VALUES ($1, $1) RETURNING id`, code).Scan(&teams[i]); err != nil {
			t.Fatal(err)
		}
	}
	roundID, err := repo.CreateRound(ctx, CreateRoundRequest{
		Name: "R", BufferSize: 2, SlotDurationMins: 10,
		Criteria: []CreateCriteriaRequest{{Name: "Idea", MinScore: 0, MaxScore: 10, Weight: 100, OrderIndex: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	panelID, err := repo.CreatePanel(ctx, CreatePanelRequest{RoundID: roundID, Name: "P", Room: "R1", JudgeIDs: []string{judge}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreatePanel(ctx, CreatePanelRequest{RoundID: roundID, Name: "Q", Room: "R2", JudgeIDs: []string{other}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ConfigureSlots(ctx, panelID, []SlotAssignment{{TeamID: teams[0], SequenceOrder: 1}, {TeamID: teams[1], SequenceOrder: 2}}, false); err != nil {
		t.Fatal(err)
	}
	if err := repo.ActivateRound(ctx, roundID); err != nil {
		t.Fatal(err)
	}
	slotOf := func(team string) string {
		var id string
		_ = pool.QueryRow(ctx, `SELECT id FROM judging_slots WHERE panel_id = $1 AND team_id = $2`, panelID, team).Scan(&id)
		return id
	}
	s1, s2 := slotOf(teams[0]), slotOf(teams[1])
	crit, _ := repo.GetRoundCriteria(ctx, roundID)
	idea := crit[0].ID
	status := func(slot string) string {
		var st string
		_ = pool.QueryRow(ctx, `SELECT status::text FROM judging_slots WHERE id = $1`, slot).Scan(&st)
		return st
	}
	lockedCount := func(slot string) int {
		var n int
		_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM judging_scores WHERE slot_id = $1 AND is_locked`, slot).Scan(&n)
		return n
	}
	unlock := func(slot, actor, role, target string) ([]string, bool, error) {
		return svc.UnlockScorecard(ctx, slot, UnlockScorecardRequest{JudgeID: target}, actor, actor+"@x", role, "ip", "ua")
	}

	if err := repo.StartSlot(ctx, s1); err != nil {
		t.Fatal(err)
	}
	if err := svc.SubmitScores(ctx, s1, judge, SubmitScoresRequest{Scores: []CriterionScoreInput{{CriterionID: idea, Score: 7}}}); err != nil {
		t.Fatal(err)
	}

	// Nothing locked yet.
	if _, _, err := unlock(s1, judge, "judge", ""); !errors.Is(err, ErrScorecardNotLocked) {
		t.Fatalf("expected ErrScorecardNotLocked, got %v", err)
	}
	if err := repo.LockJudgeScorecard(ctx, s1, judge); err != nil {
		t.Fatal(err)
	}

	// A raw UPDATE can't reopen a lock; only the unlock path can.
	if _, err := pool.Exec(ctx, `UPDATE judging_scores SET is_locked = false WHERE slot_id = $1`, s1); err == nil {
		t.Fatal("direct un-lock should be rejected by the trigger")
	}
	// A judge on another panel can't touch it.
	if _, _, err := unlock(s1, other, "judge", ""); !errors.Is(err, ErrJudgeNotAssigned) {
		t.Fatalf("expected ErrJudgeNotAssigned, got %v", err)
	}

	// The judge unlocks their own scorecard, edits, and locks again.
	if got, reopened, err := unlock(s1, judge, "judge", ""); err != nil || len(got) != 1 || reopened {
		t.Fatalf("judge unlock: %v %v %v", got, reopened, err)
	}
	if lockedCount(s1) != 0 {
		t.Fatal("scorecard should be unlocked")
	}
	lb, _ := repo.GetAdminLeaderboard(ctx, roundID)
	for _, e := range lb.Leaderboard {
		if e.TeamID == teams[0] && e.JudgesEvaluatedCount != 0 {
			t.Fatal("unlocked marks must stop counting on the leaderboard")
		}
	}
	if err := svc.SubmitScores(ctx, s1, judge, SubmitScoresRequest{Scores: []CriterionScoreInput{{CriterionID: idea, Score: 9}}}); err != nil {
		t.Fatalf("edit after unlock: %v", err)
	}
	if err := repo.LockJudgeScorecard(ctx, s1, judge); err != nil {
		t.Fatalf("re-lock: %v", err)
	}
	lb, _ = repo.GetAdminLeaderboard(ctx, roundID)
	for _, e := range lb.Leaderboard {
		if e.TeamID == teams[0] && !approx(e.TotalWeightedScore, 90) {
			t.Fatalf("edited score should count, got %v", e.TotalWeightedScore)
		}
	}

	// Complete team 1 and start team 2: unlocking team 1 would reopen it, but the room is busy.
	if err := repo.CompleteSlot(ctx, s1); err != nil {
		t.Fatal(err)
	}
	if err := repo.StartSlot(ctx, s2); err != nil {
		t.Fatal(err)
	}
	if _, _, err := unlock(s1, judge, "judge", ""); !errors.Is(err, ErrPanelBusy) {
		t.Fatalf("expected ErrPanelBusy, got %v", err)
	}
	if lockedCount(s1) != 1 || status(s1) != "completed" {
		t.Fatal("a refused unlock must change nothing")
	}

	// Once the room is free, an admin can unlock a finished team, which reopens it.
	if _, err := pool.Exec(ctx, `UPDATE judging_slots SET status = 'completed' WHERE id = $1`, s2); err != nil {
		t.Fatal(err)
	}
	got, reopened, err := unlock(s1, admin, "admin", judge)
	if err != nil || len(got) != 1 || !reopened {
		t.Fatalf("admin unlock: %v %v %v", got, reopened, err)
	}
	if status(s1) != "in_progress" || lockedCount(s1) != 0 {
		t.Fatalf("finished team should be reopened and unlocked, got %s", status(s1))
	}

	// An ended round is final.
	if _, err := pool.Exec(ctx, `UPDATE judging_rounds SET status = 'completed' WHERE id = $1`, roundID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := unlock(s1, admin, "admin", ""); !errors.Is(err, ErrRoundNotActive) {
		t.Fatalf("expected ErrRoundNotActive, got %v", err)
	}
}
