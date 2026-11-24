-- Rename users.global_role to users.role
DROP INDEX IF EXISTS idx_users_global_role;

DO $$ BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'users'
          AND column_name = 'global_role'
    ) THEN
        ALTER TABLE users RENAME COLUMN global_role TO role;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
