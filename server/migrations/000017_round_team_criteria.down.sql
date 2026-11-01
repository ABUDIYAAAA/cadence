DROP TABLE IF EXISTS judging_round_teams;
DROP INDEX IF EXISTS uq_judging_criteria_round_team;
DELETE FROM judging_criteria WHERE team_id IS NOT NULL;
ALTER TABLE judging_criteria DROP COLUMN IF EXISTS team_id;
