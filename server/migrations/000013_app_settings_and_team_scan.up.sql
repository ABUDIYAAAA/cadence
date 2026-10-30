-- Small key/value store for settings admins change from the UI.
CREATE TABLE IF NOT EXISTS app_settings (
    key VARCHAR(100) PRIMARY KEY,
    value JSONB NOT NULL,
    updated_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- When true, scanning a team leader in an attendance session marks the whole team.
ALTER TABLE scan_sessions ADD COLUMN IF NOT EXISTS mark_team BOOLEAN NOT NULL DEFAULT TRUE;
