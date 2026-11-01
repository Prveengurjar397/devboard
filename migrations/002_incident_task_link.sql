BEGIN;

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS incident_id INTEGER
    REFERENCES incidents(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_incident_id_unique
    ON tasks(incident_id)
    WHERE incident_id IS NOT NULL;

ALTER TABLE incidents
    DROP CONSTRAINT IF EXISTS incidents_status_check;

ALTER TABLE incidents
    ADD CONSTRAINT incidents_status_check
    CHECK (status IN ('open', 'acknowledged', 'in_progress', 'resolved'));

COMMIT;
