-- Drop redundant judging_slots.submission_links column; teams.project_name and teams.submission_links are the single source of truth.
ALTER TABLE judging_slots DROP COLUMN IF EXISTS submission_links;
