DROP INDEX IF EXISTS idx_scan_sessions_event_checkin_code;

ALTER TABLE scan_sessions
    DROP COLUMN IF EXISTS checkin_code_updated_at,
    DROP COLUMN IF EXISTS checkin_code;
