-- Project submissions live on the team so they can open before any judging round exists.
ALTER TABLE teams ADD COLUMN IF NOT EXISTS project_name VARCHAR(150) NULL;
ALTER TABLE teams ADD COLUMN IF NOT EXISTS submission_links JSONB NOT NULL DEFAULT '{}'::jsonb;
