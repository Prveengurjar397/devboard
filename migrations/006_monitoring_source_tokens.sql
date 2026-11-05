BEGIN;

ALTER TABLE monitoring_sources
    ADD COLUMN IF NOT EXISTS token_hash CHAR(64),
    ADD COLUMN IF NOT EXISTS token_created_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS token_revoked_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS
    idx_monitoring_sources_token_hash
ON monitoring_sources(token_hash)
WHERE token_hash IS NOT NULL;

COMMIT;
