-- ==============================================================================
-- CADENCE V2: ROTATABLE CHECK-IN CODES FOR SCAN SESSIONS
-- ==============================================================================

ALTER TABLE scan_sessions
    ADD COLUMN IF NOT EXISTS checkin_code VARCHAR(32) NULL,
    ADD COLUMN IF NOT EXISTS checkin_code_updated_at TIMESTAMPTZ NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_scan_sessions_event_checkin_code
    ON scan_sessions (event_id, UPPER(checkin_code))
    WHERE checkin_code IS NOT NULL;
