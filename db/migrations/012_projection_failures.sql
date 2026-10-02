-- Fé Pública migration 012 — durable projection failures.
-- Raw events remain the source of truth. This table records projection
-- failures without letting one malformed event starve newer valid events.

BEGIN;

CREATE TABLE IF NOT EXISTS projection_failures (
    projection      TEXT NOT NULL CHECK (projection IN ('contratos', 'cartoes', 'ceap')),
    event_id        BIGINT NOT NULL REFERENCES events(id),
    attempts        INT NOT NULL DEFAULT 1 CHECK (attempts >= 0),
    first_failed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_failed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    next_retry_at   TIMESTAMPTZ,
    terminal_at     TIMESTAMPTZ,
    resolved_at     TIMESTAMPTZ,
    error_message   TEXT NOT NULL,
    PRIMARY KEY (projection, event_id)
);

CREATE INDEX IF NOT EXISTS idx_projection_failures_pending
    ON projection_failures (projection, next_retry_at, event_id)
    WHERE resolved_at IS NULL AND terminal_at IS NULL;

COMMIT;
