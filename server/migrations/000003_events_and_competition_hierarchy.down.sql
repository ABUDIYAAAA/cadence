-- ==============================================================================
-- CADENCE V2: EVENT & COMPETITION TWO-TIER HIERARCHY ROLLBACK
-- ==============================================================================

-- 1. Restore user_competitions table
CREATE TABLE IF NOT EXISTS user_competitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_id UUID NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role competition_role NOT NULL DEFAULT 'participant',
    is_banned BOOLEAN NOT NULL DEFAULT FALSE,
    banned_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (competition_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_user_competitions_user_id ON user_competitions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_competitions_comp_role ON user_competitions(competition_id, role);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_competitions_unique_owner ON user_competitions(user_id) WHERE role = 'owner';
CREATE INDEX IF NOT EXISTS idx_user_competitions_is_banned ON user_competitions(competition_id, is_banned);

INSERT INTO user_competitions (id, competition_id, user_id, role, is_banned, banned_at, created_at, updated_at)
SELECT ue.id,
       '00000000-0000-0000-0000-000000000001'::uuid,
       ue.user_id,
       ue.role::text::competition_role,
       ue.is_banned,
       ue.banned_at,
       ue.created_at,
       ue.updated_at
FROM user_events ue
WHERE EXISTS (SELECT 1 FROM competitions c WHERE c.id = '00000000-0000-0000-0000-000000000001')
ON CONFLICT (competition_id, user_id) DO NOTHING;

-- 2. Revert Audit Logs, Notifications, App Settings, Scan Sessions, Tickets
DROP INDEX IF EXISTS idx_audit_logs_event_id;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS event_id;

ALTER TABLE notifications ADD COLUMN IF NOT EXISTS competition_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES competitions(id) ON DELETE CASCADE;
DROP INDEX IF EXISTS idx_notifications_event_id;
ALTER TABLE notifications DROP COLUMN IF EXISTS event_id;
CREATE INDEX IF NOT EXISTS idx_notifications_competition_id ON notifications(competition_id);

ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS competition_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES competitions(id) ON DELETE CASCADE;
ALTER TABLE app_settings DROP CONSTRAINT IF EXISTS app_settings_pkey;
DROP INDEX IF EXISTS idx_app_settings_event;
ALTER TABLE app_settings DROP COLUMN IF EXISTS event_id;
ALTER TABLE app_settings ADD PRIMARY KEY (competition_id, key);
CREATE INDEX IF NOT EXISTS idx_app_settings_competition ON app_settings(competition_id);

ALTER TABLE scan_sessions ADD COLUMN IF NOT EXISTS competition_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES competitions(id) ON DELETE CASCADE;
DROP INDEX IF EXISTS idx_scan_sessions_event_id;
ALTER TABLE scan_sessions DROP COLUMN IF EXISTS event_id;
CREATE INDEX IF NOT EXISTS idx_scan_sessions_competition_id ON scan_sessions(competition_id);

ALTER TABLE tickets ADD COLUMN IF NOT EXISTS competition_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES competitions(id) ON DELETE CASCADE;
ALTER TABLE tickets DROP CONSTRAINT IF EXISTS uq_tickets_event_user;
DROP INDEX IF EXISTS idx_tickets_event_id;
ALTER TABLE tickets DROP COLUMN IF EXISTS event_id;
ALTER TABLE tickets ADD CONSTRAINT tickets_competition_id_user_id_key UNIQUE (competition_id, user_id);
CREATE INDEX IF NOT EXISTS idx_tickets_competition_id ON tickets(competition_id);

-- 3. Drop team_members
DROP TABLE IF EXISTS team_members CASCADE;

-- 4. Revert competitions event_id
DROP INDEX IF EXISTS idx_competitions_event_slug;
DROP INDEX IF EXISTS idx_competitions_event_id;
ALTER TABLE competitions DROP CONSTRAINT IF EXISTS uq_competitions_event_slug;
ALTER TABLE competitions DROP COLUMN IF EXISTS event_id;
ALTER TABLE competitions ADD CONSTRAINT competitions_slug_key UNIQUE (slug);
CREATE INDEX IF NOT EXISTS idx_competitions_slug ON competitions(slug);

-- 5. Drop user_events and events
DROP TABLE IF EXISTS user_events CASCADE;
DROP TABLE IF EXISTS events CASCADE;
DROP TYPE IF EXISTS event_role CASCADE;
