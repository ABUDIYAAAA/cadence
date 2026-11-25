-- ==============================================================================
-- CADENCE V2: ROBUST TICKET EMAIL DELIVERY TRACKING
-- ==============================================================================

ALTER TABLE tickets
    ADD COLUMN IF NOT EXISTS email_status VARCHAR(32) NOT NULL DEFAULT 'pending'
        CHECK (email_status IN ('pending', 'queued', 'sent', 'failed', 'skipped_domain')),
    ADD COLUMN IF NOT EXISTS email_queued_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS email_error TEXT NULL,
    ADD COLUMN IF NOT EXISTS email_attempts INT NOT NULL DEFAULT 0;

UPDATE tickets
SET email_status = 'sent',
    email_attempts = GREATEST(email_attempts, 1)
WHERE email_sent_at IS NOT NULL AND email_status = 'pending';

CREATE INDEX IF NOT EXISTS idx_tickets_event_email_status ON tickets(event_id, email_status);
