BEGIN;

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_legacy BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO users (id, name, email, password_hash, is_legacy)
VALUES (1, 'Legacy Workspace', 'legacy-owner@opsmind.invalid', 'disabled-login', TRUE)
ON CONFLICT (id) DO NOTHING;

SELECT setval(
    pg_get_serial_sequence('users', 'id'),
    GREATEST((SELECT COALESCE(MAX(id), 1) FROM users), 1)
);

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS owner_id INTEGER;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'projects_owner_id_fkey'
          AND conrelid = 'projects'::regclass
    ) THEN
        ALTER TABLE projects
            ADD CONSTRAINT projects_owner_id_fkey
            FOREIGN KEY (owner_id) REFERENCES users(id)
            ON DELETE RESTRICT;
    END IF;
END $$;

UPDATE projects SET owner_id = 1 WHERE owner_id IS NULL;
ALTER TABLE projects ALTER COLUMN owner_id SET NOT NULL;

ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS owner_id INTEGER;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'incidents_owner_id_fkey'
          AND conrelid = 'incidents'::regclass
    ) THEN
        ALTER TABLE incidents
            ADD CONSTRAINT incidents_owner_id_fkey
            FOREIGN KEY (owner_id) REFERENCES users(id)
            ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_projects_owner_id ON projects(owner_id);
CREATE INDEX IF NOT EXISTS idx_incidents_owner_id ON incidents(owner_id);

COMMIT;
