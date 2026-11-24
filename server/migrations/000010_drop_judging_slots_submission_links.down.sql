ALTER TABLE judging_slots ADD COLUMN IF NOT EXISTS submission_links JSONB NOT NULL DEFAULT '{}'::jsonb;
