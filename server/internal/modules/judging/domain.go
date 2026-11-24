package judging

import (
	"math"
	"time"
)

// NormalizeCriterionScore computes the weighted score contribution of a raw score in [minScore, maxScore]
// according to the canonical rubric formula: ((score - min) / (max - min)) * weight.
// When maxScore <= minScore, the full weight is awarded.
func NormalizeCriterionScore(score float64, minScore, maxScore int, weight float64) float64 {
	if maxScore > minScore {
		return ((score - float64(minScore)) / float64(maxScore-minScore)) * weight
	}
	return weight
}

// PitchTimerState represents the pure calculated countdown state of a judging slot pitch.
type PitchTimerState struct {
	SlotDurationSeconds int
	PitchEnded          bool
	IsPaused            bool
	RemainingSeconds    *int
	TargetEndTime       *time.Time
}

// ComputePitchTimerState calculates the live pitch clock state from pure timestamps and durations
// without requiring database access.
func ComputePitchTimerState(
	status string,
	slotDurationMins int,
	extraSeconds int,
	pausedTotalMs int64,
	actualStartTime *time.Time,
	pausedAt *time.Time,
	pitchEndedAt *time.Time,
	serverTime time.Time,
) PitchTimerState {
	durSecs := slotDurationMins * 60
	pitchEnded := pitchEndedAt != nil
	isPaused := pausedAt != nil && !pitchEnded

	state := PitchTimerState{
		SlotDurationSeconds: durSecs,
		PitchEnded:          pitchEnded,
		IsPaused:            isPaused,
	}

	if actualStartTime != nil && status == "in_progress" {
		allowed := time.Duration(durSecs+extraSeconds) * time.Second
		paused := time.Duration(pausedTotalMs) * time.Millisecond
		clockNow := serverTime
		if pitchEndedAt != nil {
			clockNow = *pitchEndedAt
		} else if pausedAt != nil {
			clockNow = *pausedAt
		}
		remaining := int((allowed - (clockNow.Sub(*actualStartTime) - paused)).Seconds())
		state.RemainingSeconds = &remaining
		if !isPaused && !pitchEnded {
			targetEnd := serverTime.Add(time.Duration(remaining) * time.Second)
			state.TargetEndTime = &targetEnd
		}
	}

	return state
}

// AssignCompetitionRanks assigns 1-based competition ranks (1, 2, 2, 4...) to entries ordered by descending score.
// Entries with judgesEvaluatedCount == 0 receive rank 0.
func AssignCompetitionRanks(entries []AdminLeaderboardEntry) {
	scoredSoFar := 0
	rank := 0
	var prevScore float64
	for i := range entries {
		if entries[i].JudgesEvaluatedCount <= 0 {
			entries[i].Rank = 0
			continue
		}
		scoredSoFar++
		if scoredSoFar == 1 || math.Abs(entries[i].TotalWeightedScore-prevScore) > 1e-9 {
			rank = scoredSoFar
		}
		prevScore = entries[i].TotalWeightedScore
		entries[i].Rank = rank
	}
}
