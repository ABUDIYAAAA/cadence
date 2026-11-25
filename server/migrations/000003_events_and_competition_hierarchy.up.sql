-- ==============================================================================
-- CADENCE V2: EVENT & COMPETITION TWO-TIER HIERARCHY MIGRATION
-- ==============================================================================

-- 1. Create event_role enum
DO $$ BEGIN
    CREATE TYPE event_role AS ENUM (
        'owner',
        'admin',
        'volunteer',
        'judge',
        'participant'
    );
EXCEPTION WHEN duplicate_object THEN null;
END $$;

-- 2. Events (New Tenant Root)
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_events_slug ON events(slug);

-- Seed Default Initial Event
INSERT INTO events (id, slug, name, description)
VALUES ('00000000-0000-0000-0000-000000000001', 'devclub-event', 'DevClub Event', 'Default initial event')
ON CONFLICT (id) DO NOTHING;

-- If any competitions exist with IDs other than the default, ensure a corresponding event row exists for migration safety
INSERT INTO events (id, slug, name, description, created_at, updated_at)
SELECT id, slug, name, description, created_at, updated_at
FROM competitions
WHERE id <> '00000000-0000-0000-0000-000000000001'
ON CONFLICT DO NOTHING;

-- 3. User Events (Event-Scoped Membership, Banning & Single-Ownership)
CREATE TABLE IF NOT EXISTS user_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role event_role NOT NULL DEFAULT 'participant',
    is_banned BOOLEAN NOT NULL DEFAULT FALSE,
    banned_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (event_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_user_events_user_id ON user_events(user_id);
CREATE INDEX IF NOT EXISTS idx_user_events_event_role ON user_events(event_id, role);
CREATE INDEX IF NOT EXISTS idx_user_events_is_banned ON user_events(event_id, is_banned);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_events_unique_owner ON user_events(user_id) WHERE role = 'owner';

-- Migrate existing user_competitions into user_events
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'user_competitions') THEN
        INSERT INTO user_events (id, event_id, user_id, role, is_banned, banned_at, created_at, updated_at)
        SELECT uc.id,
               CASE WHEN EXISTS (SELECT 1 FROM events e WHERE e.id = uc.competition_id) THEN uc.competition_id ELSE '00000000-0000-0000-0000-000000000001'::uuid END,
               uc.user_id,
               uc.role::text::event_role,
               COALESCE(uc.is_banned, FALSE),
               uc.banned_at,
               uc.created_at,
               uc.updated_at
        FROM user_competitions uc
        ON CONFLICT (event_id, user_id) DO NOTHING;

        DROP TABLE user_competitions CASCADE;
    END IF;
END $$;

-- 4. Encapsulate Competitions within Events
ALTER TABLE competitions
ADD COLUMN IF NOT EXISTS event_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES events(id) ON DELETE CASCADE;

-- Drop global slug unique constraint on competitions and scope slug uniqueness per event
ALTER TABLE competitions DROP CONSTRAINT IF EXISTS competitions_slug_key;
DROP INDEX IF EXISTS idx_competitions_slug;
ALTER TABLE competitions DROP CONSTRAINT IF EXISTS uq_competitions_event_slug;
ALTER TABLE competitions ADD CONSTRAINT uq_competitions_event_slug UNIQUE (event_id, slug);
CREATE INDEX IF NOT EXISTS idx_competitions_event_id ON competitions(event_id);
CREATE INDEX IF NOT EXISTS idx_competitions_event_slug ON competitions(event_id, slug);

-- 5. Team Members (Multi-Competition Team Membership per User)
CREATE TABLE IF NOT EXISTS team_members (
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    competition_id UUID NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (team_id, user_id),
    CONSTRAINT uq_team_members_competition_user UNIQUE (competition_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_team_members_user_id ON team_members(user_id);
CREATE INDEX IF NOT EXISTS idx_team_members_comp_user ON team_members(competition_id, user_id);

-- Backfill team_members from existing users.team_id
INSERT INTO team_members (team_id, user_id, competition_id, joined_at)
SELECT u.team_id, u.id, t.competition_id, u.created_at
FROM users u
JOIN teams t ON t.id = u.team_id
WHERE u.team_id IS NOT NULL AND u.deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- 6. Re-scope Tickets, Scan Sessions, App Settings, Notifications, and Audit Logs to Event
-- 6a. Tickets
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS event_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES events(id) ON DELETE CASCADE;
ALTER TABLE tickets DROP CONSTRAINT IF EXISTS tickets_competition_id_user_id_key;
ALTER TABLE tickets DROP CONSTRAINT IF EXISTS tickets_user_id_key;
DROP INDEX IF EXISTS idx_tickets_competition_id;
ALTER TABLE tickets DROP COLUMN IF EXISTS competition_id;
ALTER TABLE tickets DROP CONSTRAINT IF EXISTS uq_tickets_event_user;
ALTER TABLE tickets ADD CONSTRAINT uq_tickets_event_user UNIQUE (event_id, user_id);
CREATE INDEX IF NOT EXISTS idx_tickets_event_id ON tickets(event_id);

-- 6b. Scan Sessions
ALTER TABLE scan_sessions ADD COLUMN IF NOT EXISTS event_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES events(id) ON DELETE CASCADE;
DROP INDEX IF EXISTS idx_scan_sessions_competition_id;
ALTER TABLE scan_sessions DROP COLUMN IF EXISTS competition_id;
CREATE INDEX IF NOT EXISTS idx_scan_sessions_event_id ON scan_sessions(event_id);

-- 6c. App Settings
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS event_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES events(id) ON DELETE CASCADE;
ALTER TABLE app_settings DROP CONSTRAINT IF EXISTS app_settings_pkey;
DROP INDEX IF EXISTS idx_app_settings_competition;
ALTER TABLE app_settings DROP COLUMN IF EXISTS competition_id;
ALTER TABLE app_settings ADD PRIMARY KEY (event_id, key);
CREATE INDEX IF NOT EXISTS idx_app_settings_event ON app_settings(event_id);

-- 6d. Notifications
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS event_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES events(id) ON DELETE CASCADE;
DROP INDEX IF EXISTS idx_notifications_competition_id;
ALTER TABLE notifications DROP COLUMN IF EXISTS competition_id;
CREATE INDEX IF NOT EXISTS idx_notifications_event_id ON notifications(event_id);

-- 6e. Audit Logs
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS event_id UUID NULL REFERENCES events(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_audit_logs_event_id ON audit_logs(event_id);
