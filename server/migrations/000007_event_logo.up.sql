-- ==============================================================================
-- CADENCE V2: EVENT PHOTO / LOGO SUPPORT
-- ==============================================================================

ALTER TABLE events
    ADD COLUMN IF NOT EXISTS logo_url TEXT NULL;
