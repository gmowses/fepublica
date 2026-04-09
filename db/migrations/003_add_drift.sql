-- Fé Pública migration 003 — Observatório M1: drift detector
-- Adds diff_runs (audit trail) and change_events (flat log of detected changes).
-- See docs/superpowers/specs/2026-04-09-observatorio-design.md for rationale.

BEGIN;

CREATE TABLE IF NOT EXISTS diff_runs (
    id               BIGSERIAL PRIMARY KEY,
    source_id        TEXT NOT NULL REFERENCES sources(id),
    snapshot_a_id    BIGINT NOT NULL REFERENCES snapshots(id),
    snapshot_b_id    BIGINT NOT NULL REFERENCES snapshots(id),
    added_count      INT NOT NULL DEFAULT 0,
    removed_count    INT NOT NULL DEFAULT 0,
    modified_count   INT NOT NULL DEFAULT 0,
    ran_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    duration_ms      INT NOT NULL,
    UNIQUE (snapshot_a_id, snapshot_b_id)
);
CREATE INDEX IF NOT EXISTS idx_diff_runs_source_ran
    ON diff_runs (source_id, ran_at DESC);
CREATE INDEX IF NOT EXISTS idx_diff_runs_snapshot_b
    ON diff_runs (snapshot_b_id);

CREATE TABLE IF NOT EXISTS change_events (
    id                 BIGSERIAL PRIMARY KEY,
    diff_run_id        BIGINT NOT NULL REFERENCES diff_runs(id),
    source_id          TEXT NOT NULL REFERENCES sources(id),
    ente_id            TEXT,  -- REFERENCES entes(id) once entes table exists (M3)
    external_id        TEXT NOT NULL,
    change_type        TEXT NOT NULL CHECK (change_type IN ('added','removed','modified')),
    content_hash_a     BYTEA,
    content_hash_b     BYTEA,
    detected_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    severity           TEXT NOT NULL DEFAULT 'info' CHECK (severity IN ('info','warn','alert')),
    published_rss      BOOLEAN NOT NULL DEFAULT FALSE,
    published_telegram BOOLEAN NOT NULL DEFAULT FALSE,
    published_mastodon BOOLEAN NOT NULL DEFAULT FALSE,
    published_webhook  BOOLEAN NOT NULL DEFAULT FALSE,
    published_email    BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_change_events_source_detected
    ON change_events (source_id, detected_at DESC);
CREATE INDEX IF NOT EXISTS idx_change_events_ente_detected
    ON change_events (ente_id, detected_at DESC) WHERE ente_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_change_events_external
    ON change_events (external_id);
CREATE INDEX IF NOT EXISTS idx_change_events_severity_detected
    ON change_events (severity, detected_at DESC) WHERE severity != 'info';
CREATE INDEX IF NOT EXISTS idx_change_events_unpublished_rss
    ON change_events (detected_at) WHERE published_rss = FALSE;

-- diff_runs is append-only for the content fields; only counts update allowed
-- is the initial insert. Prevent UPDATE and DELETE entirely.
CREATE OR REPLACE FUNCTION diff_runs_immutable()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'diff_runs are append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS diff_runs_no_update ON diff_runs;
CREATE TRIGGER diff_runs_no_update BEFORE UPDATE ON diff_runs
    FOR EACH ROW EXECUTE FUNCTION diff_runs_immutable();

DROP TRIGGER IF EXISTS diff_runs_no_delete ON diff_runs;
CREATE TRIGGER diff_runs_no_delete BEFORE DELETE ON diff_runs
    FOR EACH ROW EXECUTE FUNCTION diff_runs_immutable();

-- change_events allows UPDATE only for published_* flags and ente_id (nullable
-- to set-once). Everything else is immutable.
CREATE OR REPLACE FUNCTION change_events_limit_update()
RETURNS trigger AS $$
BEGIN
    IF OLD.id                 IS DISTINCT FROM NEW.id                 OR
       OLD.diff_run_id        IS DISTINCT FROM NEW.diff_run_id        OR
       OLD.source_id          IS DISTINCT FROM NEW.source_id          OR
       OLD.external_id        IS DISTINCT FROM NEW.external_id        OR
       OLD.change_type        IS DISTINCT FROM NEW.change_type        OR
       OLD.content_hash_a     IS DISTINCT FROM NEW.content_hash_a     OR
       OLD.content_hash_b     IS DISTINCT FROM NEW.content_hash_b     OR
       OLD.detected_at        IS DISTINCT FROM NEW.detected_at
    THEN
        RAISE EXCEPTION 'change_events: only severity, published_* and ente_id may be updated';
    END IF;
    -- ente_id is set-once (nullable to set, never back to null or overwritten).
    IF OLD.ente_id IS NOT NULL AND NEW.ente_id IS DISTINCT FROM OLD.ente_id THEN
        RAISE EXCEPTION 'change_events.ente_id is set-once';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS change_events_limit_update_trg ON change_events;
CREATE TRIGGER change_events_limit_update_trg BEFORE UPDATE ON change_events
    FOR EACH ROW EXECUTE FUNCTION change_events_limit_update();

DROP TRIGGER IF EXISTS change_events_no_delete ON change_events;
CREATE TRIGGER change_events_no_delete BEFORE DELETE ON change_events
    FOR EACH ROW EXECUTE FUNCTION diff_runs_immutable();

COMMIT;
