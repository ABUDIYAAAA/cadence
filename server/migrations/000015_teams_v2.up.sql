-- Teams v2: participants create teams and others join with a code.
-- All existing team and judging data is intentionally dropped.
-- DELETE (not TRUNCATE ... CASCADE) so users are not truncated through users.team_id.
UPDATE users SET team_id = NULL WHERE team_id IS NOT NULL;
DELETE FROM notifications WHERE target_type = 'team';
DELETE FROM judging_rounds; -- cascades to criteria, panels, slots, scores and analytics
DELETE FROM teams;

ALTER TABLE teams ADD COLUMN join_code VARCHAR(8) NOT NULL;
ALTER TABLE teams ADD COLUMN is_frozen BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE teams ADD COLUMN max_size INT NULL CHECK (max_size IS NULL OR max_size >= 1);
ALTER TABLE teams ADD COLUMN created_by UUID NULL REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE teams ADD CONSTRAINT teams_join_code_key UNIQUE (join_code);
