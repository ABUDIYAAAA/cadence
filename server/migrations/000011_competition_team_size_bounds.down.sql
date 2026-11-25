ALTER TABLE competitions
    DROP CONSTRAINT IF EXISTS chk_competitions_team_size_bounds,
    DROP COLUMN IF EXISTS max_team_size,
    DROP COLUMN IF EXISTS min_team_size;
