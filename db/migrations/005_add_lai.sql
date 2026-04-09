-- Fé Pública migration 005 — Observatório M5: LAI crawler tables.

BEGIN;

CREATE TABLE IF NOT EXISTS lai_checks (
    id              BIGSERIAL PRIMARY KEY,
    ente_id         TEXT NOT NULL REFERENCES entes(id),
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    target_url      TEXT NOT NULL,
    http_status     INT,
    response_ms     INT,
    ssl_valid       BOOLEAN,
    ssl_expires_at  TIMESTAMPTZ,
    portal_loads    BOOLEAN,
    html_size_bytes INT,
    terms_found     JSONB NOT NULL DEFAULT '{}'::jsonb,
    required_links  JSONB NOT NULL DEFAULT '{}'::jsonb,
    html_archive_key TEXT,
    errors          TEXT[],
    tier_at_check   INT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_lai_checks_ente_checked
    ON lai_checks (ente_id, checked_at DESC);
CREATE INDEX IF NOT EXISTS idx_lai_checks_checked
    ON lai_checks (checked_at DESC);

CREATE TABLE IF NOT EXISTS lai_scores (
    ente_id         TEXT PRIMARY KEY REFERENCES entes(id),
    score           NUMERIC(5,2) NOT NULL DEFAULT 0,
    last_check_id   BIGINT REFERENCES lai_checks(id),
    last_calculated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    components      JSONB NOT NULL DEFAULT '{}'::jsonb
);

COMMIT;
