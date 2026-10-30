-- Append-only log of every score change and lock, used for analytics and audits of judging behaviour.
CREATE TABLE IF NOT EXISTS judging_score_events (
    id BIGSERIAL PRIMARY KEY,
    round_id UUID NOT NULL REFERENCES judging_rounds(id) ON DELETE CASCADE,
    panel_id UUID NOT NULL REFERENCES judging_panels(id) ON DELETE CASCADE,
    slot_id UUID NOT NULL REFERENCES judging_slots(id) ON DELETE CASCADE,
    judge_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    criterion_id UUID NULL REFERENCES judging_criteria(id) ON DELETE CASCADE,
    event_type VARCHAR(20) NOT NULL CHECK (event_type IN ('score_set', 'score_changed', 'locked')),
    old_score NUMERIC(5,2) NULL,
    new_score NUMERIC(5,2) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_judging_score_events_round ON judging_score_events(round_id, created_at);
CREATE INDEX IF NOT EXISTS idx_judging_score_events_judge ON judging_score_events(judge_id, created_at);
CREATE INDEX IF NOT EXISTS idx_judging_score_events_slot ON judging_score_events(slot_id);

-- One row per judge per team evaluation: how long grading took and what was given.
CREATE TABLE IF NOT EXISTS judging_judge_metrics (
    slot_id UUID NOT NULL REFERENCES judging_slots(id) ON DELETE CASCADE,
    judge_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    round_id UUID NOT NULL REFERENCES judging_rounds(id) ON DELETE CASCADE,
    panel_id UUID NOT NULL REFERENCES judging_panels(id) ON DELETE CASCADE,
    slot_started_at TIMESTAMPTZ NULL,
    first_score_at TIMESTAMPTZ NULL,
    locked_at TIMESTAMPTZ NULL,
    -- first score entered -> scorecard locked
    grading_ms BIGINT NULL,
    -- pitch start -> scorecard locked, excluding paused time
    slot_ms BIGINT NULL,
    edit_count INT NOT NULL DEFAULT 0,
    criteria_count INT NOT NULL DEFAULT 0,
    score_total NUMERIC(8,2) NULL,
    score_avg NUMERIC(6,3) NULL,
    -- mean of per-criterion (score - min) / (max - min), 0..1, comparable across rubrics
    score_pct NUMERIC(6,5) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (slot_id, judge_id)
);

CREATE INDEX IF NOT EXISTS idx_judging_judge_metrics_round ON judging_judge_metrics(round_id);
CREATE INDEX IF NOT EXISTS idx_judging_judge_metrics_judge ON judging_judge_metrics(judge_id);
