DROP INDEX IF EXISTS idx_tickets_event_email_status;

ALTER TABLE tickets
    DROP COLUMN IF EXISTS email_attempts,
    DROP COLUMN IF EXISTS email_error,
    DROP COLUMN IF EXISTS email_queued_at,
    DROP COLUMN IF EXISTS email_status;
