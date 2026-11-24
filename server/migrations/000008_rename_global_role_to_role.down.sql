DROP INDEX IF EXISTS idx_users_role;

DO $$ BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'users'
          AND column_name = 'role'
    ) THEN
        ALTER TABLE users RENAME COLUMN role TO global_role;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_global_role ON users(global_role);
