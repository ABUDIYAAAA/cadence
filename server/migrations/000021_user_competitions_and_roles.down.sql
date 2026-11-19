-- Re-create user_role enum
DO $$ BEGIN
    CREATE TYPE user_role AS ENUM (
        'admin',
        'judge',
        'volunteer',
        'participant'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Add role column back to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS role user_role NOT NULL DEFAULT 'participant';

-- Restore users.role from user_competitions if present
UPDATE users u
SET role = CASE 
    WHEN uc.role::text = 'owner' THEN 'admin'::user_role
    WHEN uc.role::text IN ('admin', 'judge', 'volunteer', 'participant') THEN uc.role::text::user_role
    ELSE 'participant'::user_role
END
FROM user_competitions uc
WHERE u.id = uc.user_id;

CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

-- Drop user_competitions and competitions
DROP TABLE IF EXISTS user_competitions;
DROP TABLE IF EXISTS competitions;
DROP TYPE IF EXISTS competition_role;
