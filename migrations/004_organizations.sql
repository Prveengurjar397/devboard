BEGIN;

CREATE TABLE IF NOT EXISTS organizations (
    id SERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS organization_members (
    id SERIAL PRIMARY KEY,
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(30) NOT NULL DEFAULT 'developer'
        CHECK (role IN ('company_admin', 'developer', 'tester', 'devops')),
    can_access_incidents BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, user_id)
);

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS platform_role VARCHAR(30) NOT NULL DEFAULT 'user';

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS organization_id INTEGER;

ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS organization_id INTEGER;

INSERT INTO organizations (name, slug)
VALUES ('OpsMind Workspace', 'opsmind-workspace')
ON CONFLICT (slug) DO NOTHING;

UPDATE projects
SET organization_id = (
    SELECT id FROM organizations WHERE slug = 'opsmind-workspace'
)
WHERE organization_id IS NULL;

UPDATE incidents
SET organization_id = (
    SELECT id FROM organizations WHERE slug = 'opsmind-workspace'
)
WHERE organization_id IS NULL;

INSERT INTO organization_members
    (organization_id, user_id, role, can_access_incidents)
SELECT DISTINCT o.id, u.id, 'company_admin', TRUE
FROM organizations o
JOIN users u ON u.id = 2
WHERE o.slug = 'opsmind-workspace'
ON CONFLICT (organization_id, user_id) DO NOTHING;

UPDATE users
SET platform_role = 'platform_admin'
WHERE id = 2;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'projects_organization_id_fkey'
          AND conrelid = 'projects'::regclass
    ) THEN
        ALTER TABLE projects
        ADD CONSTRAINT projects_organization_id_fkey
        FOREIGN KEY (organization_id) REFERENCES organizations(id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'incidents_organization_id_fkey'
          AND conrelid = 'incidents'::regclass
    ) THEN
        ALTER TABLE incidents
        ADD CONSTRAINT incidents_organization_id_fkey
        FOREIGN KEY (organization_id) REFERENCES organizations(id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_projects_organization_id
    ON projects(organization_id);

CREATE INDEX IF NOT EXISTS idx_incidents_organization_id
    ON incidents(organization_id);

CREATE INDEX IF NOT EXISTS idx_org_members_user
    ON organization_members(user_id);

COMMIT;
