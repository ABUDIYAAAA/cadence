package judging

import (
	"math"
	"testing"
	"time"
)

func TestNormalizeCriterionScore(t *testing.T) {
	t.Run("standard range [0..10] with weight 50", func(t *testing.T) {
		got := NormalizeCriterionScore(8.0, 0, 10, 50.0)
		if math.Abs(got-40.0) > 1e-9 {
			t.Fatalf("expected 40.0, got %.6f", got)
		}
	})

	t.Run("non-zero min_score [2..10] with weight 50", func(t *testing.T) {
		got := NormalizeCriterionScore(6.0, 2, 10, 50.0)
		if math.Abs(got-25.0) > 1e-9 {
			t.Fatalf("expected 25.0, got %.6f", got)
		}
	})

	t.Run("degenerate range maxScore == minScore awards full weight", func(t *testing.T) {
		got := NormalizeCriterionScore(5.0, 5, 5, 30.0)
		if math.Abs(got-30.0) > 1e-9 {
			t.Fatalf("expected 30.0, got %.6f", got)
		}
	})
}

func TestComputePitchTimerState(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	started := now.Add(-4 * time.Minute) // 240s elapsed

	t.Run("running pitch subtracts paused time and adds extra seconds", func(t *testing.T) {
		// 10 min (600s) + 60s extra = 660s allowed.
		// 240s wall-clock elapsed - 30s paused = 210s active pitch time.
		// Remaining = 660 - 210 = 450s.
		st := ComputePitchTimerState("in_progress", 10, 60, 30_000, &started, nil, nil, now)
		if st.IsPaused || st.PitchEnded {
			t.Fatalf("expected running pitch state, got %+v", st)
		}
		if st.RemainingSeconds == nil || *st.RemainingSeconds != 450 {
			t.Fatalf("expected remaining=450s, got %v", st.RemainingSeconds)
		}
		if st.TargetEndTime == nil || !st.TargetEndTime.Equal(now.Add(450*time.Second)) {
			t.Fatalf("expected TargetEndTime=%v, got %v", now.Add(450*time.Second), st.TargetEndTime)
		}
	})

	t.Run("paused pitch freezes clock at pausedAt and omits TargetEndTime", func(t *testing.T) {
		pausedAt := now.Add(-1 * time.Minute) // paused after 3m (180s)
		st := ComputePitchTimerState("in_progress", 10, 0, 0, &started, &pausedAt, nil, now)
		if !st.IsPaused || st.PitchEnded {
			t.Fatalf("expected paused state, got %+v", st)
		}
		if st.RemainingSeconds == nil || *st.RemainingSeconds != 420 {
			t.Fatalf("expected remaining=420s at pause point, got %v", st.RemainingSeconds)
		}
		if st.TargetEndTime != nil {
			t.Fatalf("expected nil TargetEndTime while paused, got %v", st.TargetEndTime)
		}
	})

	t.Run("ended pitch freezes clock at pitchEndedAt and marks PitchEnded=true", func(t *testing.T) {
		endedAt := now.Add(-2 * time.Minute) // ended after 2m (120s)
		st := ComputePitchTimerState("in_progress", 10, 0, 0, &started, nil, &endedAt, now)
		if !st.PitchEnded || st.IsPaused {
			t.Fatalf("expected ended state, got %+v", st)
		}
		if st.RemainingSeconds == nil || *st.RemainingSeconds != 480 {
			t.Fatalf("expected remaining=480s at pitch end, got %v", st.RemainingSeconds)
		}
	})
}

func TestAssignCompetitionRanks(t *testing.T) {
	entries := []AdminLeaderboardEntry{
		{TeamName: "Alpha", JudgesEvaluatedCount: 2, TotalWeightedScore: 92.5},
		{TeamName: "Beta", JudgesEvaluatedCount: 2, TotalWeightedScore: 85.0},
		{TeamName: "Gamma", JudgesEvaluatedCount: 2, TotalWeightedScore: 85.0},
		{TeamName: "Delta", JudgesEvaluatedCount: 1, TotalWeightedScore: 70.0},
		{TeamName: "Unscored", JudgesEvaluatedCount: 0, TotalWeightedScore: 0.0},
	}
	AssignCompetitionRanks(entries)

	want := []int{1, 2, 2, 4, 0}
	for i, w := range want {
		if entries[i].Rank != w {
			t.Fatalf("entry %d (%s): expected rank %d, got %d", i, entries[i].TeamName, w, entries[i].Rank)
		}
	}
}
