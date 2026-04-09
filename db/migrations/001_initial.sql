-- Fé Pública initial schema
-- All event data is strictly append-only enforced by triggers.

BEGIN;

CREATE TABLE IF NOT EXISTS sources (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    base_url    TEXT NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS snapshots (
    id                 BIGSERIAL PRIMARY KEY,
    source_id          TEXT NOT NULL REFERENCES sources(id),
    collected_at       TIMESTAMPTZ NOT NULL,
    api_version        TEXT,
    record_count       INT NOT NULL DEFAULT 0,
    bytes_size         BIGINT NOT NULL DEFAULT 0,
    merkle_root        BYTEA,
    merkle_computed_at TIMESTAMPTZ,
    collector_version  TEXT NOT NULL,
    notes              TEXT,
    UNIQUE (source_id, collected_at)
);
CREATE INDEX IF NOT EXISTS idx_snapshots_source_collected
    ON snapshots (source_id, collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_snapshots_no_merkle
    ON snapshots (id) WHERE merkle_root IS NULL;

CREATE TABLE IF NOT EXISTS events (
    id             BIGSERIAL PRIMARY KEY,
    snapshot_id    BIGINT NOT NULL REFERENCES snapshots(id),
    source_id      TEXT NOT NULL REFERENCES sources(id),
    external_id    TEXT NOT NULL,
    content_hash   BYTEA NOT NULL,
    canonical_json JSONB NOT NULL,
    collected_at   TIMESTAMPTZ NOT NULL,
    UNIQUE (snapshot_id, external_id)
);
CREATE INDEX IF NOT EXISTS idx_events_source_external
    ON events (source_id, external_id);
CREATE INDEX IF NOT EXISTS idx_events_content_hash
    ON events (content_hash);

CREATE TABLE IF NOT EXISTS anchors (
    id           BIGSERIAL PRIMARY KEY,
    snapshot_id  BIGINT NOT NULL REFERENCES snapshots(id),
    calendar_url TEXT NOT NULL,
    submitted_at TIMESTAMPTZ NOT NULL,
    receipt      BYTEA NOT NULL,
    upgraded     BOOLEAN NOT NULL DEFAULT FALSE,
    upgraded_at  TIMESTAMPTZ,
    block_height INT,
    UNIQUE (snapshot_id, calendar_url)
);
CREATE INDEX IF NOT EXISTS idx_anchors_snapshot
    ON anchors (snapshot_id);
CREATE INDEX IF NOT EXISTS idx_anchors_pending_upgrade
    ON anchors (id) WHERE upgraded = FALSE;

-- Immutability triggers for events
CREATE OR REPLACE FUNCTION events_immutable()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'events are append-only; UPDATE and DELETE are forbidden';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS events_no_update ON events;
CREATE TRIGGER events_no_update BEFORE UPDATE ON events
    FOR EACH ROW EXECUTE FUNCTION events_immutable();

DROP TRIGGER IF EXISTS events_no_delete ON events;
CREATE TRIGGER events_no_delete BEFORE DELETE ON events
    FOR EACH ROW EXECUTE FUNCTION events_immutable();

-- Snapshots: allow UPDATE only for merkle_root and merkle_computed_at going from NULL to set
CREATE OR REPLACE FUNCTION snapshots_limit_update()
RETURNS trigger AS $$
BEGIN
    IF OLD.id              IS DISTINCT FROM NEW.id              OR
       OLD.source_id       IS DISTINCT FROM NEW.source_id       OR
       OLD.collected_at    IS DISTINCT FROM NEW.collected_at    OR
       OLD.api_version     IS DISTINCT FROM NEW.api_version     OR
       OLD.record_count    IS DISTINCT FROM NEW.record_count    OR
       OLD.bytes_size      IS DISTINCT FROM NEW.bytes_size      OR
       OLD.collector_version IS DISTINCT FROM NEW.collector_version
    THEN
        RAISE EXCEPTION 'snapshots: only merkle_root, merkle_computed_at, notes may be updated';
    END IF;
    IF OLD.merkle_root IS NOT NULL AND NEW.merkle_root IS DISTINCT FROM OLD.merkle_root THEN
        RAISE EXCEPTION 'snapshots: merkle_root is set-once';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS snapshots_limit_update_trg ON snapshots;
CREATE TRIGGER snapshots_limit_update_trg BEFORE UPDATE ON snapshots
    FOR EACH ROW EXECUTE FUNCTION snapshots_limit_update();

DROP TRIGGER IF EXISTS snapshots_no_delete ON snapshots;
CREATE TRIGGER snapshots_no_delete BEFORE DELETE ON snapshots
    FOR EACH ROW EXECUTE FUNCTION events_immutable();

-- Anchors: allow update only for upgrade-related fields
CREATE OR REPLACE FUNCTION anchors_limit_update()
RETURNS trigger AS $$
BEGIN
    IF OLD.id           IS DISTINCT FROM NEW.id           OR
       OLD.snapshot_id  IS DISTINCT FROM NEW.snapshot_id  OR
       OLD.calendar_url IS DISTINCT FROM NEW.calendar_url OR
       OLD.submitted_at IS DISTINCT FROM NEW.submitted_at
    THEN
        RAISE EXCEPTION 'anchors: immutable fields cannot change';
    END IF;
    IF OLD.upgraded = TRUE AND NEW.upgraded = FALSE THEN
        RAISE EXCEPTION 'anchors: upgraded cannot go from true to false';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS anchors_limit_update_trg ON anchors;
CREATE TRIGGER anchors_limit_update_trg BEFORE UPDATE ON anchors
    FOR EACH ROW EXECUTE FUNCTION anchors_limit_update();

DROP TRIGGER IF EXISTS anchors_no_delete ON anchors;
CREATE TRIGGER anchors_no_delete BEFORE DELETE ON anchors
    FOR EACH ROW EXECUTE FUNCTION events_immutable();

-- Seed sources
INSERT INTO sources (id, name, base_url, description) VALUES
    ('ceis', 'Cadastro de Empresas Inidôneas e Suspensas',
     'https://api.portaldatransparencia.gov.br/api-de-dados/ceis',
     'Lista de empresas impedidas de contratar com o poder público, mantida pela CGU.'),
    ('cnep', 'Cadastro Nacional de Empresas Punidas',
     'https://api.portaldatransparencia.gov.br/api-de-dados/cnep',
     'Lista de empresas penalizadas pela Lei Anticorrupção (Lei 12.846/2013), mantida pela CGU.')
ON CONFLICT (id) DO NOTHING;

COMMIT;
