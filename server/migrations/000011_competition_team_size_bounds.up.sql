ALTER TABLE competitions
    ADD COLUMN IF NOT EXISTS min_team_size INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS max_team_size INT NOT NULL DEFAULT 4;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_competitions_team_size_bounds'
    ) THEN
        ALTER TABLE competitions
            ADD CONSTRAINT chk_competitions_team_size_bounds
            CHECK (min_team_size >= 1 AND max_team_size >= min_team_size AND max_team_size <= 100);
    END IF;
END $$;
