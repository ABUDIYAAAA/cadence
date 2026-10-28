-- Add device_id to refresh_tokens for cryptographic device binding and session management
ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS device_id VARCHAR(64) NOT NULL DEFAULT 'legacy';

-- Remove default after column creation
ALTER TABLE refresh_tokens ALTER COLUMN device_id DROP DEFAULT;

-- Indexes for device-specific session lookups and revocations
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_device_id ON refresh_tokens(device_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_device ON refresh_tokens(user_id, device_id);
