-- Drop token tables
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS refresh_tokens;

-- Drop foreign key from teams to users first to avoid circular dependency
ALTER TABLE IF EXISTS teams DROP CONSTRAINT IF EXISTS teams_leader_id_fkey;

-- Drop main tables
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS teams;

-- Drop custom types
DROP TYPE IF EXISTS reset_token_purpose;
DROP TYPE IF EXISTS user_role;
