-- Per-team overrides set by an admin. NULL means "use the round's slot_duration_mins" / "no pinned start".
ALTER TABLE judging_slots
    ADD COLUMN IF NOT EXISTS duration_mins INT NULL CHECK (duration_mins IS NULL OR duration_mins > 0),
    ADD COLUMN IF NOT EXISTS scheduled_start_time TIMESTAMPTZ NULL;
