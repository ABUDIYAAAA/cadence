-- ==============================================================================
-- CADENCE V2: COMPETITION-SCOPED BANNING SCHEMA ROLLBACK
-- ==============================================================================

DROP INDEX IF EXISTS idx_user_competitions_is_banned;

ALTER TABLE user_competitions
DROP COLUMN IF EXISTS banned_at,
DROP COLUMN IF EXISTS is_banned;
