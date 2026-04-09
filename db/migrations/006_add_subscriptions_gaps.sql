-- Fé Pública migration 006 — Observatório M6+M7: subscriptions (webhook) and gaps catalog.
-- Email subscriptions and gap evidence are scaffolded minimally for v0.7/v0.8.

BEGIN;

CREATE TABLE IF NOT EXISTS subscriptions_webhook (
    id              BIGSERIAL PRIMARY KEY,
    url             TEXT NOT NULL,
    secret          TEXT NOT NULL,
    filter_sources  TEXT[] NOT NULL DEFAULT '{}',
    filter_severity TEXT NOT NULL DEFAULT 'warn',
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    owner_contact   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    failure_count   INT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_webhook_active ON subscriptions_webhook (active);

CREATE TABLE IF NOT EXISTS subscriptions_email (
    id                BIGSERIAL PRIMARY KEY,
    email             TEXT NOT NULL UNIQUE,
    confirmed_at      TIMESTAMPTZ,
    confirm_token     TEXT,
    unsubscribe_token TEXT NOT NULL,
    filter_sources    TEXT[] NOT NULL DEFAULT '{}',
    filter_severity   TEXT NOT NULL DEFAULT 'warn',
    cadence           TEXT NOT NULL DEFAULT 'daily' CHECK (cadence IN ('realtime','daily','weekly')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_sent_at      TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_email_confirmed ON subscriptions_email (confirmed_at) WHERE confirmed_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS gap_catalog (
    id              BIGSERIAL PRIMARY KEY,
    ente_id         TEXT REFERENCES entes(id),
    source_id       TEXT REFERENCES sources(id),
    category        TEXT NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL,
    severity        TEXT NOT NULL CHECK (severity IN ('info','warn','alert')),
    legal_reference TEXT,
    status          TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','acknowledged','resolved','wont_fix')),
    first_seen_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS idx_gap_catalog_ente_status ON gap_catalog (ente_id, status);
CREATE INDEX IF NOT EXISTS idx_gap_catalog_category_status ON gap_catalog (category, status);
CREATE INDEX IF NOT EXISTS idx_gap_catalog_severity_status ON gap_catalog (severity, status);

CREATE TABLE IF NOT EXISTS gap_evidence (
    id              BIGSERIAL PRIMARY KEY,
    gap_id          BIGINT NOT NULL REFERENCES gap_catalog(id),
    kind            TEXT NOT NULL CHECK (kind IN ('change_event','lai_check','snapshot','url','note')),
    change_event_id BIGINT REFERENCES change_events(id),
    lai_check_id    BIGINT REFERENCES lai_checks(id),
    snapshot_id     BIGINT REFERENCES snapshots(id),
    url             TEXT,
    note            TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_gap_evidence_gap ON gap_evidence (gap_id);

COMMIT;
