ALTER TABLE judging_slots
    DROP COLUMN IF EXISTS paused_at,
    DROP COLUMN IF EXISTS paused_total_ms,
    DROP COLUMN IF EXISTS extra_secs;
