-- Fé Pública migration 013 — current projection views.
--
-- Projection rows are retained for every complete collection. Operational
-- reads use the latest version of each external record without deleting the
-- historical rows needed for audit and projection retries.

BEGIN;

CREATE OR REPLACE VIEW current_contratos AS
SELECT DISTINCT ON (c.external_id) c.*
FROM contratos c
JOIN snapshots s ON s.id = c.snapshot_id
WHERE s.lifecycle_status = 'complete'
ORDER BY c.external_id, s.collected_at DESC, c.event_id DESC;

CREATE OR REPLACE VIEW current_cartoes AS
SELECT DISTINCT ON (c.external_id) c.*
FROM cartoes c
JOIN snapshots s ON s.id = c.snapshot_id
WHERE s.lifecycle_status = 'complete'
ORDER BY c.external_id, s.collected_at DESC, c.event_id DESC;

CREATE OR REPLACE VIEW current_ceap AS
SELECT DISTINCT ON (c.external_id) c.*
FROM ceap c
JOIN snapshots s ON s.id = c.snapshot_id
WHERE s.lifecycle_status = 'complete'
ORDER BY c.external_id, s.collected_at DESC, c.event_id DESC;

COMMIT;
