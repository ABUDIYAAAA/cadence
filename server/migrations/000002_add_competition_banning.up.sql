-- ==============================================================================
-- CADENCE V2: COMPETITION-SCOPED BANNING SCHEMA MIGRATION
-- ==============================================================================

ALTER TABLE user_competitions
ADD COLUMN IF NOT EXISTS is_banned BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS banned_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_user_competitions_is_banned ON user_competitions(competition_id, is_banned);
