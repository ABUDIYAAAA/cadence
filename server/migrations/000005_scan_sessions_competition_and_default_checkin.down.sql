DROP TRIGGER IF EXISTS trg_events_create_default_checkin_session ON events;
DROP FUNCTION IF EXISTS fn_events_create_default_checkin_session();

DELETE FROM scan_sessions WHERE is_default = TRUE OR kind = 'check_in';

DROP INDEX IF EXISTS idx_scan_sessions_default_checkin;
DROP INDEX IF EXISTS idx_scan_sessions_competition_id;

ALTER TABLE scan_sessions DROP CONSTRAINT IF EXISTS scan_sessions_kind_check;
ALTER TABLE scan_sessions
    ADD CONSTRAINT scan_sessions_kind_check CHECK (kind IN ('attendance', 'exit'));

ALTER TABLE scan_sessions
    DROP COLUMN IF EXISTS is_default,
    DROP COLUMN IF EXISTS competition_id;
