-- Fé Pública migration 009 — R3.A: persisted forenses findings.
--
-- Until now the /forenses endpoints ran the detector SQLs on every request.
-- Persisting findings unlocks:
--   - dedup across runs (a finding is "the same" if its dedup_key matches)
--   - human curation (dismiss/confirm with audit trail)
--   - atom feed of new findings for journalists to subscribe to
--   - notification on first sight (telegram/mastodon hooks)
--   - historical view of when each pattern first appeared

BEGIN;

CREATE TABLE IF NOT EXISTS findings (
    id              BIGSERIAL PRIMARY KEY,

    -- Detector identity
    finding_type    TEXT NOT NULL,         -- 'sancionado_contratado', etc
    detector_version INTEGER NOT NULL DEFAULT 1,

    -- Stable per-finding identity. Built by the detector from the underlying
    -- IDs (e.g. contrato_id + fornecedor_ni). UNIQUE prevents duplicates.
    dedup_key       TEXT NOT NULL,

    -- Surfacing fields (denormalized for cheap reads/feeds)
    severity        TEXT NOT NULL,          -- 'high'|'medium'|'low'
    title           TEXT NOT NULL,
    subject         TEXT NOT NULL,
    valor           NUMERIC(18, 2),

    -- Full evidence as JSON (the same shape returned by the API)
    evidence        JSONB NOT NULL,

    -- Optional link back to the entity in the SPA
    link            TEXT,

    -- Lifecycle
    first_seen_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Curation (manual)
    dismissed_at    TIMESTAMPTZ,
    dismissed_by    TEXT,
    dismissed_note  TEXT,
    confirmed_at    TIMESTAMPTZ,
    confirmed_by    TEXT,

    -- Notification status (so we don't spam channels on every run)
    notified_at     TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_findings_dedup ON findings (dedup_key);
CREATE INDEX IF NOT EXISTS idx_findings_type_severity ON findings (finding_type, severity);
CREATE INDEX IF NOT EXISTS idx_findings_first_seen ON findings (first_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_findings_pending_notify ON findings (id) WHERE notified_at IS NULL AND severity = 'high';
CREATE INDEX IF NOT EXISTS idx_findings_active ON findings (severity, first_seen_at DESC) WHERE dismissed_at IS NULL;

COMMIT;
