-- Judging round status enum
DO $$ BEGIN
    CREATE TYPE judging_round_status AS ENUM ('draft', 'active', 'completed');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Judging slot status enum
DO $$ BEGIN
    CREATE TYPE judging_slot_status AS ENUM ('scheduled', 'queued', 'in_progress', 'completed', 'skipped', 'no_show');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- 1. Judging Rounds
CREATE TABLE IF NOT EXISTS judging_rounds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    status judging_round_status NOT NULL DEFAULT 'draft',
    buffer_size INT NOT NULL DEFAULT 2,
    slot_duration_mins INT NOT NULL DEFAULT 15,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 2. Judging Criteria (per round)
CREATE TABLE IF NOT EXISTS judging_criteria (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    round_id UUID NOT NULL REFERENCES judging_rounds(id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    description TEXT NULL,
    max_score INT NOT NULL DEFAULT 10,
    weight NUMERIC(5,2) NOT NULL DEFAULT 1.00,
    order_index INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 3. Judging Panels (mapped to a round and physical room)
CREATE TABLE IF NOT EXISTS judging_panels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    round_id UUID NOT NULL REFERENCES judging_rounds(id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    room VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 4. Panel Judges Mapping
CREATE TABLE IF NOT EXISTS judging_panel_judges (
    panel_id UUID NOT NULL REFERENCES judging_panels(id) ON DELETE CASCADE,
    judge_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (panel_id, judge_id)
);

-- 5. Judging Slots (sequence of teams in a panel)
CREATE TABLE IF NOT EXISTS judging_slots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    panel_id UUID NOT NULL REFERENCES judging_panels(id) ON DELETE CASCADE,
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    sequence_order INT NOT NULL,
    status judging_slot_status NOT NULL DEFAULT 'scheduled',
    estimated_start_time TIMESTAMPTZ NULL,
    actual_start_time TIMESTAMPTZ NULL,
    actual_end_time TIMESTAMPTZ NULL,
    submission_links JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (panel_id, team_id),
    UNIQUE (panel_id, sequence_order)
);

-- 6. Judging Scores (per judge, per slot, per criterion)
CREATE TABLE IF NOT EXISTS judging_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slot_id UUID NOT NULL REFERENCES judging_slots(id) ON DELETE CASCADE,
    judge_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    criterion_id UUID NOT NULL REFERENCES judging_criteria(id) ON DELETE CASCADE,
    score NUMERIC(5,2) NOT NULL DEFAULT 0.00,
    notes TEXT NULL,
    is_locked BOOLEAN NOT NULL DEFAULT FALSE,
    locked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (slot_id, judge_id, criterion_id)
);

-- Indexes for high-performance lookup
CREATE INDEX IF NOT EXISTS idx_judging_criteria_round_id ON judging_criteria(round_id);
CREATE INDEX IF NOT EXISTS idx_judging_panels_round_id ON judging_panels(round_id);
CREATE INDEX IF NOT EXISTS idx_judging_panel_judges_judge_id ON judging_panel_judges(judge_id);
CREATE INDEX IF NOT EXISTS idx_judging_slots_panel_id ON judging_slots(panel_id);
CREATE INDEX IF NOT EXISTS idx_judging_slots_team_id ON judging_slots(team_id);
CREATE INDEX IF NOT EXISTS idx_judging_slots_status ON judging_slots(status);
CREATE INDEX IF NOT EXISTS idx_judging_scores_slot_id ON judging_scores(slot_id);
CREATE INDEX IF NOT EXISTS idx_judging_scores_judge_id ON judging_scores(judge_id);
