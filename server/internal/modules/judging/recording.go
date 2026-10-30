package judging

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// recordScoreChangesTx appends score events and bumps the judge's per-slot metrics.
// It runs inside the score-save transaction, before the scores are overwritten, so the
// previous values are still readable and the log can never drift from the scores.
func recordScoreChangesTx(ctx context.Context, tx pgx.Tx, slotID, judgeID string, scores []CriterionScoreInput) error {
	var roundID, panelID string
	if err := tx.QueryRow(ctx, `
		SELECT p.round_id, p.id
		FROM judging_slots s JOIN judging_panels p ON p.id = s.panel_id
		WHERE s.id = $1`, slotID).Scan(&roundID, &panelID); err != nil {
		return fmt.Errorf("resolve slot round: %w", err)
	}

	previous := make(map[string]float64)
	rows, err := tx.Query(ctx,
		`SELECT criterion_id, score FROM judging_scores WHERE slot_id = $1 AND judge_id = $2`, slotID, judgeID)
	if err != nil {
		return fmt.Errorf("read previous scores: %w", err)
	}
	for rows.Next() {
		var id string
		var sc float64
		if err := rows.Scan(&id, &sc); err != nil {
			rows.Close()
			return fmt.Errorf("scan previous score: %w", err)
		}
		previous[id] = sc
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read previous scores: %w", err)
	}

	edits := 0
	for _, sc := range scores {
		old, existed := previous[sc.CriterionID]
		if existed && old == sc.Score {
			continue
		}
		eventType := "score_set"
		var oldScore *float64
		if existed {
			eventType = "score_changed"
			edits++
			oldScore = &old
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO judging_score_events (round_id, panel_id, slot_id, judge_id, criterion_id, event_type, old_score, new_score)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			roundID, panelID, slotID, judgeID, sc.CriterionID, eventType, oldScore, sc.Score); err != nil {
			return fmt.Errorf("record score event: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO judging_judge_metrics (slot_id, judge_id, round_id, panel_id, slot_started_at, first_score_at, edit_count)
		SELECT $1, $2, $3, $4, s.actual_start_time, CURRENT_TIMESTAMP, $5
		FROM judging_slots s WHERE s.id = $1
		ON CONFLICT (slot_id, judge_id) DO UPDATE
		SET edit_count = judging_judge_metrics.edit_count + EXCLUDED.edit_count,
		    updated_at = CURRENT_TIMESTAMP`,
		slotID, judgeID, roundID, panelID, edits); err != nil {
		return fmt.Errorf("record judge metrics: %w", err)
	}
	return nil
}

// recordLockMetricsTx finalises the judge's metrics row when they lock their scorecard.
func recordLockMetricsTx(ctx context.Context, tx pgx.Tx, slotID, judgeID string) error {
	var roundID, panelID string
	if err := tx.QueryRow(ctx, `
		SELECT p.round_id, p.id
		FROM judging_slots s JOIN judging_panels p ON p.id = s.panel_id
		WHERE s.id = $1`, slotID).Scan(&roundID, &panelID); err != nil {
		return fmt.Errorf("resolve slot round: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO judging_score_events (round_id, panel_id, slot_id, judge_id, event_type)
		VALUES ($1, $2, $3, $4, 'locked')`, roundID, panelID, slotID, judgeID); err != nil {
		return fmt.Errorf("record lock event: %w", err)
	}

	// first_score_at falls back to the lock time for cards that were scored and locked in one call.
	if _, err := tx.Exec(ctx, `
		INSERT INTO judging_judge_metrics (slot_id, judge_id, round_id, panel_id, slot_started_at, first_score_at)
		SELECT $1, $2, $3, $4, s.actual_start_time, CURRENT_TIMESTAMP
		FROM judging_slots s WHERE s.id = $1
		ON CONFLICT (slot_id, judge_id) DO NOTHING`,
		slotID, judgeID, roundID, panelID); err != nil {
		return fmt.Errorf("ensure judge metrics: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		WITH agg AS (
			SELECT COUNT(*) AS n,
			       SUM(sc.score) AS total,
			       AVG(sc.score) AS avg,
			       AVG(CASE WHEN c.max_score > c.min_score
			                THEN (sc.score - c.min_score) / (c.max_score - c.min_score)::numeric
			                ELSE 1 END) AS pct
			FROM judging_scores sc
			JOIN judging_criteria c ON c.id = sc.criterion_id
			WHERE sc.slot_id = $1 AND sc.judge_id = $2
		)
		UPDATE judging_judge_metrics m
		SET locked_at = CURRENT_TIMESTAMP,
		    grading_ms = GREATEST(0, (EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - m.first_score_at)) * 1000)::bigint),
		    slot_ms = CASE WHEN s.actual_start_time IS NULL THEN NULL ELSE GREATEST(0,
		        (EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - s.actual_start_time)) * 1000)::bigint - s.paused_total_ms) END,
		    criteria_count = agg.n,
		    score_total = agg.total,
		    score_avg = agg.avg,
		    score_pct = agg.pct,
		    updated_at = CURRENT_TIMESTAMP
		FROM agg, judging_slots s
		WHERE m.slot_id = $1 AND m.judge_id = $2 AND s.id = m.slot_id`, slotID, judgeID); err != nil {
		return fmt.Errorf("finalise judge metrics: %w", err)
	}
	return nil
}
