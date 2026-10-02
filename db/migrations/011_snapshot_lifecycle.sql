-- Fé Pública migration 011 — snapshot collection lifecycle.
--
-- Events may commit in independent batches. A snapshot is only eligible for
-- Merkle construction after its declared count is observed under its row lock.

BEGIN;

ALTER TABLE snapshots ADD COLUMN IF NOT EXISTS lifecycle_status TEXT;

-- The old update trigger predates lifecycle_status. Replace it before the
-- backfill while retaining its immutability guarantees for all audit fields.
CREATE OR REPLACE FUNCTION snapshots_limit_update()
RETURNS trigger AS $$
DECLARE
    event_total INT;
BEGIN
    IF OLD.id                IS DISTINCT FROM NEW.id OR
       OLD.source_id         IS DISTINCT FROM NEW.source_id OR
       OLD.collected_at      IS DISTINCT FROM NEW.collected_at OR
       OLD.api_version       IS DISTINCT FROM NEW.api_version OR
       OLD.record_count      IS DISTINCT FROM NEW.record_count OR
       OLD.bytes_size        IS DISTINCT FROM NEW.bytes_size OR
       OLD.collector_version IS DISTINCT FROM NEW.collector_version
    THEN
        RAISE EXCEPTION 'snapshots: audit fields are immutable';
    END IF;

    IF OLD.lifecycle_status IS DISTINCT FROM NEW.lifecycle_status THEN
        -- NULL is only possible during this migration's backfill.
        IF OLD.lifecycle_status IS NOT NULL OR NEW.lifecycle_status NOT IN ('complete', 'failed') THEN
            RAISE EXCEPTION 'snapshots: lifecycle may only transition collecting to complete or failed';
        END IF;
        IF NEW.lifecycle_status = 'complete' THEN
            SELECT COUNT(*) INTO event_total FROM events WHERE snapshot_id = NEW.id;
            IF event_total <> NEW.record_count THEN
                RAISE EXCEPTION 'snapshots: cannot complete with % events; expected %', event_total, NEW.record_count;
            END IF;
        END IF;
    END IF;

    IF OLD.merkle_root IS NOT NULL AND NEW.merkle_root IS DISTINCT FROM OLD.merkle_root THEN
        RAISE EXCEPTION 'snapshots: merkle_root is set-once';
    END IF;
    IF NEW.merkle_root IS DISTINCT FROM OLD.merkle_root THEN
        IF NEW.merkle_root IS NULL OR NEW.lifecycle_status <> 'complete' THEN
            RAISE EXCEPTION 'snapshots: only complete snapshots may receive a merkle root';
        END IF;
        SELECT COUNT(*) INTO event_total FROM events WHERE snapshot_id = NEW.id;
        IF event_total <> NEW.record_count THEN
            RAISE EXCEPTION 'snapshots: cannot set root with % events; expected %', event_total, NEW.record_count;
        END IF;
    END IF;
    IF NEW.merkle_computed_at IS DISTINCT FROM OLD.merkle_computed_at AND
       NEW.merkle_root IS NOT DISTINCT FROM OLD.merkle_root THEN
        RAISE EXCEPTION 'snapshots: merkle_computed_at changes only when setting root';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Do not infer completeness for old partial rows. Roots are retained exactly
-- as recorded, including any historic root whose event count was incomplete.
UPDATE snapshots s
SET lifecycle_status = CASE
    WHEN (SELECT COUNT(*) FROM events e WHERE e.snapshot_id = s.id) = s.record_count
        THEN 'complete'
    ELSE 'failed'
END
WHERE lifecycle_status IS NULL;

ALTER TABLE snapshots
    ALTER COLUMN lifecycle_status SET DEFAULT 'collecting',
    ALTER COLUMN lifecycle_status SET NOT NULL;

ALTER TABLE snapshots DROP CONSTRAINT IF EXISTS snapshots_lifecycle_status_check;
ALTER TABLE snapshots ADD CONSTRAINT snapshots_lifecycle_status_check
    CHECK (lifecycle_status IN ('collecting', 'complete', 'failed'));

CREATE OR REPLACE FUNCTION snapshots_limit_update()
RETURNS trigger AS $$
DECLARE
    event_total INT;
BEGIN
    IF OLD.id                IS DISTINCT FROM NEW.id OR
       OLD.source_id         IS DISTINCT FROM NEW.source_id OR
       OLD.collected_at      IS DISTINCT FROM NEW.collected_at OR
       OLD.api_version       IS DISTINCT FROM NEW.api_version OR
       OLD.record_count      IS DISTINCT FROM NEW.record_count OR
       OLD.bytes_size        IS DISTINCT FROM NEW.bytes_size OR
       OLD.collector_version IS DISTINCT FROM NEW.collector_version
    THEN
        RAISE EXCEPTION 'snapshots: audit fields are immutable';
    END IF;
    IF OLD.lifecycle_status IS DISTINCT FROM NEW.lifecycle_status AND
       (OLD.lifecycle_status <> 'collecting' OR NEW.lifecycle_status NOT IN ('complete', 'failed')) THEN
        RAISE EXCEPTION 'snapshots: lifecycle may only transition collecting to complete or failed';
    END IF;
    IF OLD.lifecycle_status IS DISTINCT FROM NEW.lifecycle_status AND NEW.lifecycle_status = 'complete' THEN
        SELECT COUNT(*) INTO event_total FROM events WHERE snapshot_id = NEW.id;
        IF event_total <> NEW.record_count THEN
            RAISE EXCEPTION 'snapshots: cannot complete with % events; expected %', event_total, NEW.record_count;
        END IF;
    END IF;
    IF OLD.merkle_root IS NOT NULL AND NEW.merkle_root IS DISTINCT FROM OLD.merkle_root THEN
        RAISE EXCEPTION 'snapshots: merkle_root is set-once';
    END IF;
    IF NEW.merkle_root IS DISTINCT FROM OLD.merkle_root THEN
        IF NEW.merkle_root IS NULL OR OLD.lifecycle_status <> 'complete' THEN
            RAISE EXCEPTION 'snapshots: only complete snapshots may receive a merkle root';
        END IF;
        SELECT COUNT(*) INTO event_total FROM events WHERE snapshot_id = NEW.id;
        IF event_total <> NEW.record_count THEN
            RAISE EXCEPTION 'snapshots: cannot set root with % events; expected %', event_total, NEW.record_count;
        END IF;
    END IF;
    IF NEW.merkle_computed_at IS DISTINCT FROM OLD.merkle_computed_at AND
       NEW.merkle_root IS NOT DISTINCT FROM OLD.merkle_root THEN
        RAISE EXCEPTION 'snapshots: merkle_computed_at changes only when setting root';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION events_require_collecting_snapshot()
RETURNS trigger AS $$
BEGIN
    -- This lock conflicts with both FOR UPDATE and the NO KEY UPDATE lock an
    -- ordinary UPDATE takes. A batch that began first finishes before any
    -- lifecycle seal; a later batch sees the seal.
    PERFORM 1
    FROM snapshots
    WHERE id = NEW.snapshot_id
      AND lifecycle_status = 'collecting'
      AND merkle_root IS NULL
    FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'events: snapshot % is not accepting new events', NEW.snapshot_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS events_require_collecting_snapshot_trg ON events;
CREATE TRIGGER events_require_collecting_snapshot_trg
    BEFORE INSERT ON events
    FOR EACH ROW EXECUTE FUNCTION events_require_collecting_snapshot();

CREATE INDEX IF NOT EXISTS idx_snapshots_complete_no_merkle
    ON snapshots (id) WHERE lifecycle_status = 'complete' AND merkle_root IS NULL;

COMMIT;
