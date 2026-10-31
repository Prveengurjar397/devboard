CREATE TABLE IF NOT EXISTS incidents (
    id SERIAL PRIMARY KEY,
    title VARCHAR(300) NOT NULL,
    description TEXT,
    container_name VARCHAR(255),
    severity VARCHAR(10) NOT NULL DEFAULT 'medium'
        CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    status VARCHAR(20) NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'acknowledged', 'resolved')),
    root_cause TEXT,
    suggested_fix TEXT,
    logs TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_incidents_status
    ON incidents(status);

CREATE INDEX IF NOT EXISTS idx_incidents_severity
    ON incidents(severity);

CREATE INDEX IF NOT EXISTS idx_incidents_created_at
    ON incidents(created_at DESC);

DROP TRIGGER IF EXISTS incidents_touch_updated_at ON incidents;

CREATE TRIGGER incidents_touch_updated_at
    BEFORE UPDATE ON incidents
    FOR EACH ROW
    EXECUTE FUNCTION touch_updated_at();
