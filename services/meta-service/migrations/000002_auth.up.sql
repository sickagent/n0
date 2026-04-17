CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_jti TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_agents_user ON agents(user_id);

-- Link existing entities to a default root user
INSERT INTO users (id, email, password_hash, role)
VALUES ('00000000-0000-0000-0000-000000000001', 'admin@n0.local', '', 'admin')
ON CONFLICT (email) DO NOTHING;

-- Add user_id to workspaces
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE CASCADE;
UPDATE workspaces SET user_id = '00000000-0000-0000-0000-000000000001' WHERE user_id IS NULL;
ALTER TABLE workspaces ALTER COLUMN user_id SET NOT NULL;

-- Add user_id to connections
ALTER TABLE connections ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE CASCADE;
UPDATE connections SET user_id = '00000000-0000-0000-0000-000000000001' WHERE user_id IS NULL;
ALTER TABLE connections ALTER COLUMN user_id SET NOT NULL;

-- Make tenant_id nullable for backward compat in main tables
ALTER TABLE workspaces ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE connections ALTER COLUMN tenant_id DROP NOT NULL;
