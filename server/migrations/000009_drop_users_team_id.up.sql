-- Drop redundant legacy users.team_id column; team_members is the single source of truth for team membership.
DROP INDEX IF EXISTS idx_users_team_id;
ALTER TABLE users DROP COLUMN IF EXISTS team_id;
