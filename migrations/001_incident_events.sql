CREATE TABLE IF NOT EXISTS incident_events (
    id BIGSERIAL PRIMARY KEY,

    incident_id INTEGER NOT NULL
        REFERENCES incidents(id) ON DELETE CASCADE,

    event_type VARCHAR(50) NOT NULL,

    message TEXT NOT NULL,

    actor_type VARCHAR(30) NOT NULL DEFAULT 'system',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_incident_events_incident_created
    ON incident_events(incident_id, created_at ASC, id ASC);
