-- Add deleted_at to users for soft-deletion and audit preservation
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
