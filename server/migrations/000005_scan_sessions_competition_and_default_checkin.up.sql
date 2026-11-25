-- ==============================================================================
-- CADENCE V2: DEFAULT CHECK-IN SCAN SESSION & SINGLE-COMPETITION SCAN LINKING
-- ==============================================================================

-- 1. Allow scan_sessions to optionally link to a single competition and mark default check-in session
ALTER TABLE scan_sessions
    ADD COLUMN IF NOT EXISTS competition_id UUID NULL REFERENCES competitions(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT FALSE;

-- Expand kind check constraint to include 'check_in'
ALTER TABLE scan_sessions DROP CONSTRAINT IF EXISTS scan_sessions_kind_check;
ALTER TABLE scan_sessions
    ADD CONSTRAINT scan_sessions_kind_check CHECK (kind IN ('check_in', 'attendance', 'exit'));

CREATE INDEX IF NOT EXISTS idx_scan_sessions_competition_id ON scan_sessions(competition_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_scan_sessions_default_checkin ON scan_sessions(event_id) WHERE is_default = TRUE;

-- 2. Seed default Check-In scan session for all existing events
INSERT INTO scan_sessions (event_id, name, kind, is_default, is_open, mark_team)
SELECT e.id, 'Entry Check-In', 'check_in', TRUE, TRUE, FALSE
FROM events e
WHERE NOT EXISTS (
    SELECT 1 FROM scan_sessions s WHERE s.event_id = e.id AND s.is_default = TRUE
);

-- 3. Backfill existing checked-in tickets into their event's default Check-In scan session
INSERT INTO scan_events (session_id, ticket_id, scanned_by, scanned_at)
SELECT s.id, t.id, t.checked_in_by, COALESCE(t.checked_in_at, t.updated_at)
FROM tickets t
JOIN scan_sessions s ON s.event_id = t.event_id AND s.is_default = TRUE
WHERE t.is_checked_in = TRUE
ON CONFLICT (session_id, ticket_id) DO NOTHING;

-- 4. Automatically create the default Check-In scan session whenever a new event is inserted
CREATE OR REPLACE FUNCTION fn_events_create_default_checkin_session()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO scan_sessions (event_id, name, kind, is_default, is_open, mark_team)
    VALUES (NEW.id, 'Entry Check-In', 'check_in', TRUE, TRUE, FALSE)
    ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_events_create_default_checkin_session ON events;
CREATE TRIGGER trg_events_create_default_checkin_session
AFTER INSERT ON events
FOR EACH ROW EXECUTE FUNCTION fn_events_create_default_checkin_session();
