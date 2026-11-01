-- A per-team criterion may carry a pre-awarded score (from the CSV import). Such a criterion is
-- not judged: judges never see or score it, and its score counts straight into the team total.
ALTER TABLE judging_criteria ADD COLUMN IF NOT EXISTS awarded_score NUMERIC(6,2) NULL;
