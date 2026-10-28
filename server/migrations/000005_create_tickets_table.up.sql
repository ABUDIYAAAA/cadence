CREATE TABLE IF NOT EXISTS tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    ticket_code VARCHAR(100) NOT NULL UNIQUE,
    is_checked_in BOOLEAN NOT NULL DEFAULT FALSE,
    checked_in_at TIMESTAMPTZ NULL,
    checked_in_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    email_sent_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_tickets_ticket_code ON tickets(ticket_code);
CREATE INDEX IF NOT EXISTS idx_tickets_checked_in ON tickets(is_checked_in);
CREATE INDEX IF NOT EXISTS idx_tickets_checked_in_at ON tickets(checked_in_at DESC);
CREATE INDEX IF NOT EXISTS idx_tickets_checked_in_by ON tickets(checked_in_by);
