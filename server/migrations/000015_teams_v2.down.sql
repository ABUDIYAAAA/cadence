ALTER TABLE teams DROP CONSTRAINT IF EXISTS teams_join_code_key;
ALTER TABLE teams DROP COLUMN IF EXISTS created_by;
ALTER TABLE teams DROP COLUMN IF EXISTS max_size;
ALTER TABLE teams DROP COLUMN IF EXISTS is_frozen;
ALTER TABLE teams DROP COLUMN IF EXISTS join_code;
DELETE FROM app_settings WHERE key IN ('teams.frozen_all', 'teams.max_size');
