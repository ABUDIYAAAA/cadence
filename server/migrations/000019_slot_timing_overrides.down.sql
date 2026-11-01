ALTER TABLE judging_slots
    DROP COLUMN IF EXISTS duration_mins,
    DROP COLUMN IF EXISTS scheduled_start_time;
