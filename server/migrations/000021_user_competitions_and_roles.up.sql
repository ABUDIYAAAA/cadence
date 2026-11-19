-- 1. Create competition role enum
DO $$ BEGIN
    CREATE TYPE competition_role AS ENUM (
        'owner',
        'admin',
        'volunteer',
        'judge',
        'participant'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- 2. Create competitions table
CREATE TABLE IF NOT EXISTS competitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_competitions_slug ON competitions(slug);

-- 3. Create user_competitions mapping table
CREATE TABLE IF NOT EXISTS user_competitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    competition_id UUID NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    role competition_role NOT NULL DEFAULT 'participant',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (competition_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_competitions_user_id ON user_competitions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_competitions_comp_role ON user_competitions(competition_id, role);

-- Enforce that a user may only be the owner of one competition at a time
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_competitions_unique_owner 
ON user_competitions (user_id) 
WHERE role = 'owner';

-- 4. Seed default competition for existing event
INSERT INTO competitions (id, slug, name, description)
VALUES ('00000000-0000-0000-0000-000000000001', 'devclub-hack', 'DevClub Hackathon', 'Default initial competition')
ON CONFLICT (id) DO NOTHING;

-- 5. Backfill existing users into user_competitions from users.role if role column exists
DO $$ 
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name = 'users' AND column_name = 'role'
    ) THEN
        INSERT INTO user_competitions (user_id, competition_id, role)
        SELECT id, '00000000-0000-0000-0000-000000000001', role::text::competition_role
        FROM users
        ON CONFLICT (competition_id, user_id) DO NOTHING;

        DROP INDEX IF EXISTS idx_users_role;
        ALTER TABLE users DROP COLUMN role;
        DROP TYPE IF EXISTS user_role;
    END IF;
END $$;
