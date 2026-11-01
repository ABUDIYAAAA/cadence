-- Per-team criteria: a criterion with team_id set applies only to that team's slots.
ALTER TABLE judging_criteria
    ADD COLUMN IF NOT EXISTS team_id UUID NULL REFERENCES teams(id) ON DELETE CASCADE;

CREATE UNIQUE INDEX IF NOT EXISTS uq_judging_criteria_round_team
    ON judging_criteria (round_id, team_id) WHERE team_id IS NOT NULL;

-- Roster: the teams going into a round (seeded from the CSV import).
CREATE TABLE IF NOT EXISTS judging_round_teams (
    round_id UUID NOT NULL REFERENCES judging_rounds(id) ON DELETE CASCADE,
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (round_id, team_id)
);
