package judging

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSlotTiming_OverridesAndChaining(t *testing.T) {
	pool, _ := newTestJudgingPool(t, 19)
	repo := NewRepository(pool)
	ctx := context.Background()

	var judgeID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, is_active, is_password_configured)
		VALUES ('judge.st@cadence.local', 'Judge ST', 'judge', true, true) RETURNING id`).Scan(&judgeID); err != nil {
		t.Fatalf("seed judge: %v", err)
	}
	teams := make([]string, 3)
	for i, code := range []string{"TIMEA001", "TIMEB002", "TIMEC003"} {
		if err := pool.QueryRow(ctx, `INSERT INTO teams (name, join_code) VALUES ($1, $1) RETURNING id`, code).Scan(&teams[i]); err != nil {
			t.Fatalf("seed team: %v", err)
		}
	}
	roundID, err := repo.CreateRound(ctx, CreateRoundRequest{
		Name: "R", BufferSize: 2, SlotDurationMins: 10,
		Criteria: []CreateCriteriaRequest{{Name: "C", MinScore: 0, MaxScore: 10, Weight: 100, OrderIndex: 1}},
	})
	if err != nil {
		t.Fatalf("create round: %v", err)
	}
	panelID, err := repo.CreatePanel(ctx, CreatePanelRequest{RoundID: roundID, Name: "P", Room: "R1", JudgeIDs: []string{judgeID}})
	if err != nil {
		t.Fatalf("create panel: %v", err)
	}
	if err := repo.ConfigureSlots(ctx, panelID, []SlotAssignment{
		{TeamID: teams[0], SequenceOrder: 1}, {TeamID: teams[1], SequenceOrder: 2}, {TeamID: teams[2], SequenceOrder: 3},
	}, false); err != nil {
		t.Fatalf("configure slots: %v", err)
	}

	slotIDs := make([]string, 3)
	rows, err := pool.Query(ctx, `SELECT id FROM judging_slots WHERE panel_id = $1 ORDER BY sequence_order`, panelID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; rows.Next(); i++ {
		_ = rows.Scan(&slotIDs[i])
	}
	rows.Close()

	// Team 2 gets a pinned start and a 20 minute pitch; team 3 should chain after it.
	pin := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	dur := 20
	if err := repo.UpdateSlotTiming(ctx, slotIDs[1], UpdateSlotTimingRequest{DurationMins: &dur, StartTime: &pin}); err != nil {
		t.Fatalf("update timing: %v", err)
	}

	var est2, est3 time.Time
	var effective int
	_ = pool.QueryRow(ctx, `SELECT estimated_start_time FROM judging_slots WHERE id = $1`, slotIDs[1]).Scan(&est2)
	_ = pool.QueryRow(ctx, `SELECT estimated_start_time FROM judging_slots WHERE id = $1`, slotIDs[2]).Scan(&est3)
	if !est2.Equal(pin) {
		t.Fatalf("pinned start not honored: got %v want %v", est2, pin)
	}
	if want := pin.Add(20 * time.Minute); !est3.Equal(want) {
		t.Fatalf("next team should chain after 20m pitch: got %v want %v", est3, want)
	}

	// The judge's timer uses the team's own length.
	d, err := repo.GetJudgeSlotDetails(ctx, slotIDs[1], judgeID)
	if err != nil {
		t.Fatalf("slot details: %v", err)
	}
	effective = d.SlotDurationSeconds
	if effective != 20*60 {
		t.Fatalf("timer length = %ds, want 1200s", effective)
	}

	// Clearing falls back to the round default.
	if err := repo.UpdateSlotTiming(ctx, slotIDs[1], UpdateSlotTimingRequest{}); err != nil {
		t.Fatalf("clear timing: %v", err)
	}
	d, _ = repo.GetJudgeSlotDetails(ctx, slotIDs[1], judgeID)
	if d.SlotDurationSeconds != 10*60 {
		t.Fatalf("cleared timer length = %ds, want 600s", d.SlotDurationSeconds)
	}

	// Finished teams are locked.
	if _, err := pool.Exec(ctx, `UPDATE judging_slots SET status = 'completed' WHERE id = $1`, slotIDs[0]); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateSlotTiming(ctx, slotIDs[0], UpdateSlotTimingRequest{DurationMins: &dur}); !errors.Is(err, ErrSlotTimingLocked) {
		t.Fatalf("expected ErrSlotTimingLocked, got %v", err)
	}
}

func TestRoundDuration_ChangeAndReset(t *testing.T) {
	pool, _ := newTestJudgingPool(t, 19)
	repo := NewRepository(pool)
	ctx := context.Background()

	var judgeID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, is_active, is_password_configured)
		VALUES ('judge.rd@cadence.local', 'Judge RD', 'judge', true, true) RETURNING id`).Scan(&judgeID); err != nil {
		t.Fatal(err)
	}
	teams := make([]string, 2)
	for i, code := range []string{"RDTEAM01", "RDTEAM02"} {
		if err := pool.QueryRow(ctx, `INSERT INTO teams (name, join_code) VALUES ($1, $1) RETURNING id`, code).Scan(&teams[i]); err != nil {
			t.Fatal(err)
		}
	}
	roundID, err := repo.CreateRound(ctx, CreateRoundRequest{
		Name: "R", BufferSize: 2, SlotDurationMins: 10,
		Criteria: []CreateCriteriaRequest{{Name: "C", MinScore: 0, MaxScore: 10, Weight: 100, OrderIndex: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	panelID, err := repo.CreatePanel(ctx, CreatePanelRequest{RoundID: roundID, Name: "P", Room: "R1", JudgeIDs: []string{judgeID}})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ConfigureSlots(ctx, panelID, []SlotAssignment{{TeamID: teams[0], SequenceOrder: 1}, {TeamID: teams[1], SequenceOrder: 2}}, false); err != nil {
		t.Fatal(err)
	}
	var s1, s2 string
	_ = pool.QueryRow(ctx, `SELECT id FROM judging_slots WHERE panel_id=$1 AND sequence_order=1`, panelID).Scan(&s1)
	_ = pool.QueryRow(ctx, `SELECT id FROM judging_slots WHERE panel_id=$1 AND sequence_order=2`, panelID).Scan(&s2)
	custom := 25
	if err := repo.UpdateSlotTiming(ctx, s1, UpdateSlotTimingRequest{DurationMins: &custom}); err != nil {
		t.Fatal(err)
	}

	// Without reset: round default changes, the custom team keeps its own length.
	if _, err := repo.UpdateRoundSlotDuration(ctx, roundID, 12, false); err != nil {
		t.Fatalf("change duration: %v", err)
	}
	d1, _ := repo.GetJudgeSlotDetails(ctx, s1, judgeID)
	d2, _ := repo.GetJudgeSlotDetails(ctx, s2, judgeID)
	if d1.SlotDurationSeconds != 25*60 || d2.SlotDurationSeconds != 12*60 {
		t.Fatalf("got %ds/%ds, want 1500s/720s", d1.SlotDurationSeconds, d2.SlotDurationSeconds)
	}

	// With reset: the custom team follows the new default too.
	reset, err := repo.UpdateRoundSlotDuration(ctx, roundID, 8, true)
	if err != nil || reset != 1 {
		t.Fatalf("reset: n=%d err=%v", reset, err)
	}
	d1, _ = repo.GetJudgeSlotDetails(ctx, s1, judgeID)
	if d1.SlotDurationSeconds != 8*60 {
		t.Fatalf("after reset got %ds, want 480s", d1.SlotDurationSeconds)
	}

	if _, err := pool.Exec(ctx, `UPDATE judging_rounds SET status='completed' WHERE id=$1`, roundID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateRoundSlotDuration(ctx, roundID, 5, false); !errors.Is(err, ErrRoundCompleted) {
		t.Fatalf("expected ErrRoundCompleted, got %v", err)
	}
}
