BEGIN;

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS created_by INTEGER
    REFERENCES users(id) ON DELETE SET NULL;

UPDATE tasks t
SET created_by = p.owner_id
FROM projects p
WHERE t.project_id = p.id
  AND t.created_by IS NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_created_by
    ON tasks(created_by);

CREATE INDEX IF NOT EXISTS idx_members_org_user
    ON organization_members(organization_id, user_id);

-- Platform owner is assigned only to the designated existing account.
UPDATE users
SET platform_role = 'user'
WHERE id <> 3
  AND platform_role = 'platform_admin';

UPDATE users
SET platform_role = 'platform_admin'
WHERE id = 3
  AND lower(email) = 'gujjarpraveen397@gmail.com';

COMMIT;
