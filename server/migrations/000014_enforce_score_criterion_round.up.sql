-- 1. Purge any legacy/invalid score rows whose criterion does not belong to the slot's round
--    or whose score falls outside the criterion's allowed [min_score, max_score] bounds.
DELETE FROM judging_scores sc
WHERE NOT EXISTS (
    SELECT 1
    FROM judging_slots s
    JOIN judging_panels p ON p.id = s.panel_id
    JOIN judging_criteria c ON c.id = sc.criterion_id
    WHERE s.id = sc.slot_id
      AND c.round_id = p.round_id
      AND sc.score >= c.min_score
      AND sc.score <= c.max_score
);

-- 2. Establish composite (id, round_id) uniqueness on panels and criteria to support composite FKs.
ALTER TABLE judging_panels
    ADD CONSTRAINT uq_judging_panels_id_round UNIQUE (id, round_id);

ALTER TABLE judging_criteria
    ADD CONSTRAINT uq_judging_criteria_id_round UNIQUE (id, round_id);

-- 3. Add round_id to judging_slots and bind (panel_id, round_id) to judging_panels(id, round_id).
ALTER TABLE judging_slots
    ADD COLUMN IF NOT EXISTS round_id UUID NULL;

UPDATE judging_slots s
SET round_id = p.round_id
FROM judging_panels p
WHERE p.id = s.panel_id AND s.round_id IS NULL;

CREATE OR REPLACE FUNCTION fn_judging_slots_sync_round_id()
RETURNS TRIGGER AS $$
DECLARE
    v_panel_round_id UUID;
BEGIN
    SELECT round_id INTO v_panel_round_id
    FROM judging_panels
    WHERE id = NEW.panel_id;

    IF v_panel_round_id IS NULL THEN
        RAISE EXCEPTION 'judging panel % does not exist', NEW.panel_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    IF NEW.round_id IS NULL THEN
        NEW.round_id := v_panel_round_id;
    ELSIF NEW.round_id <> v_panel_round_id THEN
        RAISE EXCEPTION 'slot round_id % does not match panel round_id %', NEW.round_id, v_panel_round_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_judging_slots_sync_round_id ON judging_slots;
CREATE TRIGGER trg_judging_slots_sync_round_id
BEFORE INSERT OR UPDATE OF panel_id, round_id ON judging_slots
FOR EACH ROW
EXECUTE FUNCTION fn_judging_slots_sync_round_id();

ALTER TABLE judging_slots
    ALTER COLUMN round_id SET NOT NULL,
    ADD CONSTRAINT uq_judging_slots_id_round UNIQUE (id, round_id),
    ADD CONSTRAINT fk_judging_slots_panel_round
        FOREIGN KEY (panel_id, round_id) REFERENCES judging_panels(id, round_id) ON DELETE CASCADE;

-- 4. Add round_id to judging_scores and enforce declarative composite foreign keys to BOTH
--    judging_slots(id, round_id) and judging_criteria(id, round_id), plus a trigger for score bounds
--    and lock immutability.
ALTER TABLE judging_scores
    ADD COLUMN IF NOT EXISTS round_id UUID NULL;

UPDATE judging_scores sc
SET round_id = s.round_id
FROM judging_slots s
WHERE s.id = sc.slot_id AND sc.round_id IS NULL;

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

DROP TRIGGER IF EXISTS trg_judging_scores_enforce_integrity ON judging_scores;
CREATE TRIGGER trg_judging_scores_enforce_integrity
BEFORE INSERT OR UPDATE ON judging_scores
FOR EACH ROW
EXECUTE FUNCTION fn_judging_scores_enforce_integrity();

ALTER TABLE judging_scores
    ALTER COLUMN round_id SET NOT NULL,
    ADD CONSTRAINT fk_judging_scores_slot_round
        FOREIGN KEY (slot_id, round_id) REFERENCES judging_slots(id, round_id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_judging_scores_criterion_round
        FOREIGN KEY (criterion_id, round_id) REFERENCES judging_criteria(id, round_id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_judging_scores_round_id ON judging_scores(round_id);
