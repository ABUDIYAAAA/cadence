-- Named scan sessions beyond entry check-in: repeatable attendance rounds and exit scans.
CREATE TABLE IF NOT EXISTS scan_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(150) NOT NULL,
    kind VARCHAR(20) NOT NULL CHECK (kind IN ('attendance', 'exit')),
    is_open BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMPTZ NULL
);

-- One scan per ticket per session; the unique pair is what makes duplicate detection atomic.
CREATE TABLE IF NOT EXISTS scan_events (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES scan_sessions(id) ON DELETE CASCADE,
    ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    scanned_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    scanned_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (session_id, ticket_id)
);

CREATE INDEX IF NOT EXISTS idx_scan_events_ticket ON scan_events(ticket_id);
CREATE INDEX IF NOT EXISTS idx_scan_events_session_time ON scan_events(session_id, scanned_at DESC);

-- Set when an admin releases the pass to the participant dashboard or emails it, whichever is first.
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS released_at TIMESTAMPTZ NULL;
UPDATE tickets SET released_at = COALESCE(email_sent_at, created_at) WHERE released_at IS NULL;
