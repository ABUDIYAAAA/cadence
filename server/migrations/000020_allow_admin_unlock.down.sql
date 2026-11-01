DELETE FROM judging_score_events WHERE event_type = 'unlocked';
ALTER TABLE judging_score_events DROP CONSTRAINT IF EXISTS judging_score_events_event_type_check;
ALTER TABLE judging_score_events ADD CONSTRAINT judging_score_events_event_type_check
    CHECK (event_type IN ('score_set', 'score_changed', 'locked'));

CREATE OR REPLACE FUNCTION fn_judging_scores_enforce_integrity()
RETURNS TRIGGER AS $$
DECLARE
    v_slot_round_id UUID;
    v_crit_round_id UUID;
    v_min_score INT;
    v_max_score INT;
BEGIN
    IF TG_OP = 'UPDATE' AND OLD.is_locked = TRUE AND (
        NEW.is_locked = FALSE OR
        NEW.score IS DISTINCT FROM OLD.score OR
        NEW.notes IS DISTINCT FROM OLD.notes OR
        NEW.criterion_id IS DISTINCT FROM OLD.criterion_id OR
        NEW.slot_id IS DISTINCT FROM OLD.slot_id OR
        NEW.judge_id IS DISTINCT FROM OLD.judge_id
    ) THEN
        RAISE EXCEPTION 'scorecard is locked and cannot be edited'
            USING ERRCODE = 'check_violation';
    END IF;

    SELECT round_id INTO v_slot_round_id
    FROM judging_slots
    WHERE id = NEW.slot_id;

    IF v_slot_round_id IS NULL THEN
        RAISE EXCEPTION 'judging slot % does not exist', NEW.slot_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    IF NEW.round_id IS NULL THEN
        NEW.round_id := v_slot_round_id;
    ELSIF NEW.round_id <> v_slot_round_id THEN
        RAISE EXCEPTION 'score round_id % does not match slot round_id %', NEW.round_id, v_slot_round_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    SELECT round_id, min_score, max_score
    INTO v_crit_round_id, v_min_score, v_max_score
    FROM judging_criteria
    WHERE id = NEW.criterion_id;

    IF v_crit_round_id IS NULL THEN
        RAISE EXCEPTION 'judging criterion % does not exist', NEW.criterion_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    IF v_crit_round_id <> NEW.round_id THEN
        RAISE EXCEPTION 'criterion % belongs to round %, not slot round %', NEW.criterion_id, v_crit_round_id, NEW.round_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    IF NEW.score < v_min_score OR NEW.score > v_max_score THEN
        RAISE EXCEPTION 'score % is outside allowed range [%, %] for criterion %', NEW.score, v_min_score, v_max_score, NEW.criterion_id
            USING ERRCODE = 'check_violation';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
