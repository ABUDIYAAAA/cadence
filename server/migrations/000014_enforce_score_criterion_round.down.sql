DROP INDEX IF EXISTS idx_judging_scores_round_id;

ALTER TABLE judging_scores
    DROP CONSTRAINT IF EXISTS fk_judging_scores_criterion_round,
    DROP CONSTRAINT IF EXISTS fk_judging_scores_slot_round;

DROP TRIGGER IF EXISTS trg_judging_scores_enforce_integrity ON judging_scores;
DROP FUNCTION IF EXISTS fn_judging_scores_enforce_integrity();

ALTER TABLE judging_scores
    DROP COLUMN IF EXISTS round_id;

ALTER TABLE judging_slots
    DROP CONSTRAINT IF EXISTS fk_judging_slots_panel_round,
    DROP CONSTRAINT IF EXISTS uq_judging_slots_id_round;

DROP TRIGGER IF EXISTS trg_judging_slots_sync_round_id ON judging_slots;
DROP FUNCTION IF EXISTS fn_judging_slots_sync_round_id();

ALTER TABLE judging_slots
    DROP COLUMN IF EXISTS round_id;

ALTER TABLE judging_criteria
    DROP CONSTRAINT IF EXISTS uq_judging_criteria_id_round;

ALTER TABLE judging_panels
    DROP CONSTRAINT IF EXISTS uq_judging_panels_id_round;
