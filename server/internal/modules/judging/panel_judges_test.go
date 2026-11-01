package judging

import (
	"context"
	"errors"
	"testing"
)

func TestSetPanelJudges_Rules(t *testing.T) {
	pool, _ := newTestJudgingPool(t, 20)
	repo := NewRepository(pool)
	ctx := context.Background()

	mk := func(email, role string) string {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO users (email, name, role, is_active, is_password_configured)
			VALUES ($1, $1, $2, true, true) RETURNING id`, email, role).Scan(&id); err != nil {
			t.Fatalf("seed user: %v", err)
		}
		return id
	}
	j1, j2, j3 := mk("j1@cadence.local", "judge"), mk("j2@cadence.local", "judge"), mk("j3@cadence.local", "judge")
	notJudge := mk("v1@cadence.local", "volunteer")

	var team string
	if err := pool.QueryRow(ctx, `INSERT INTO teams (name, join_code) VALUES ('PJTEAM01', 'PJTEAM01') RETURNING id`).Scan(&team); err != nil {
		t.Fatal(err)
	}
	roundID, err := repo.CreateRound(ctx, CreateRoundRequest{
		Name: "R", BufferSize: 2, SlotDurationMins: 10,
		Criteria: []CreateCriteriaRequest{{Name: "C", MinScore: 0, MaxScore: 10, Weight: 100, OrderIndex: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p1, err := repo.CreatePanel(ctx, CreatePanelRequest{RoundID: roundID, Name: "A", Room: "R1", JudgeIDs: []string{j1}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreatePanel(ctx, CreatePanelRequest{RoundID: roundID, Name: "B", Room: "R2", JudgeIDs: []string{j3}}); err != nil {
		t.Fatal(err)
	}

	added, removed, err := repo.SetPanelJudges(ctx, p1, []string{j1, j2})
	if err != nil || len(added) != 1 || len(removed) != 0 {
		t.Fatalf("add judge: added=%v removed=%v err=%v", added, removed, err)
	}
	if _, _, err := repo.SetPanelJudges(ctx, p1, nil); !errors.Is(err, ErrNoJudges) {
		t.Fatalf("expected ErrNoJudges, got %v", err)
	}
	if _, _, err := repo.SetPanelJudges(ctx, p1, []string{j1, notJudge}); !errors.Is(err, ErrNotJudges) {
		t.Fatalf("expected ErrNotJudges, got %v", err)
	}
	if _, _, err := repo.SetPanelJudges(ctx, p1, []string{j1, j3}); !errors.Is(err, ErrJudgeAssigned) {
		t.Fatalf("expected ErrJudgeAssigned, got %v", err)
	}

	// A judge who has scored in the room can't be removed.
	if err := repo.ConfigureSlots(ctx, p1, []SlotAssignment{{TeamID: team, SequenceOrder: 1}}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO judging_scores (slot_id, judge_id, criterion_id, score)
		SELECT s.id, $2, c.id, 5 FROM judging_slots s, judging_criteria c WHERE s.panel_id = $1 AND c.round_id = $3`,
		p1, j2, roundID); err != nil {
		t.Fatalf("seed score: %v", err)
	}
	if _, _, err := repo.SetPanelJudges(ctx, p1, []string{j1}); !errors.Is(err, ErrJudgeHasScores) {
		t.Fatalf("expected ErrJudgeHasScores, got %v", err)
	}
}
