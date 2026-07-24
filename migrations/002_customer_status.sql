ALTER TABLE users
ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'blocked'));

CREATE INDEX IF NOT EXISTS idx_users_role_status ON users(role, status) WHERE deleted_at IS NULL;
