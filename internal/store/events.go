package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// InsertEventParams holds one event to be inserted.
type InsertEventParams struct {
	SnapshotID    int64
	SourceID      string
	ExternalID    string
	ContentHash   []byte
	CanonicalJSON []byte
}

// insertBatchChunkSize caps how many events are queued in a single pgx.Batch.
// Keeping it bounded lets pgx stream statements to Postgres instead of
// buffering megabytes of JSON in memory, which matters for large collectors
// (CEIS with 22k+ records produces ~75 MB of raw JSON).
const insertBatchChunkSize = 500

// InsertEventsBatch inserts many events, chunked into transactions of
// insertBatchChunkSize rows each. Returns the total number of rows inserted.
// On error, previously committed chunks remain persisted — the caller can
// treat partial progress as recoverable (the snapshot row will still exist
// with its final count, and idempotent re-runs can be detected by the
// snapshots.unique(source_id, collected_at) constraint).
func (s *Store) InsertEventsBatch(ctx context.Context, events []InsertEventParams) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}

	total := 0
	for start := 0; start < len(events); start += insertBatchChunkSize {
		end := start + insertBatchChunkSize
		if end > len(events) {
			end = len(events)
		}
		chunk := events[start:end]

		inserted, err := s.insertEventsChunk(ctx, chunk)
		total += inserted
		if err != nil {
			return total, fmt.Errorf("store: chunk [%d:%d]: %w", start, end, err)
		}
	}
	return total, nil
}

func (s *Store) insertEventsChunk(ctx context.Context, events []InsertEventParams) (int, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	batch := &pgx.Batch{}
	for _, e := range events {
		batch.Queue(`
			INSERT INTO events (snapshot_id, source_id, external_id, content_hash, canonical_json, collected_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
		`, e.SnapshotID, e.SourceID, e.ExternalID, e.ContentHash, e.CanonicalJSON)
	}
	br := tx.SendBatch(ctx, batch)
	inserted := 0
	for i := 0; i < len(events); i++ {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return inserted, fmt.Errorf("insert event %d: %w", i, err)
		}
		inserted++
	}
	if err := br.Close(); err != nil {
		return inserted, fmt.Errorf("close batch: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return inserted, fmt.Errorf("commit: %w", err)
	}
	return inserted, nil
}

// ListEventsBySnapshot returns all events of a snapshot, in insertion order (by id).
func (s *Store) ListEventsBySnapshot(ctx context.Context, snapshotID int64) ([]Event, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, snapshot_id, source_id, external_id, content_hash, canonical_json, collected_at
		FROM events
		WHERE snapshot_id = $1
		ORDER BY id ASC
	`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("store: list events: %w", err)
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var ev Event
		if err := rows.Scan(
			&ev.ID, &ev.SnapshotID, &ev.SourceID, &ev.ExternalID,
			&ev.ContentHash, &ev.CanonicalJSON, &ev.CollectedAt,
		); err != nil {
			return nil, fmt.Errorf("store: scan event: %w", err)
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// GetEventByExternalID returns the event with the given external id in a snapshot.
func (s *Store) GetEventByExternalID(ctx context.Context, snapshotID int64, externalID string) (*Event, error) {
	var ev Event
	err := s.pool.QueryRow(ctx, `
		SELECT id, snapshot_id, source_id, external_id, content_hash, canonical_json, collected_at
		FROM events
		WHERE snapshot_id = $1 AND external_id = $2
	`, snapshotID, externalID).Scan(
		&ev.ID, &ev.SnapshotID, &ev.SourceID, &ev.ExternalID,
		&ev.ContentHash, &ev.CanonicalJSON, &ev.CollectedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("store: event %s not found in snapshot %d", externalID, snapshotID)
		}
		return nil, fmt.Errorf("store: get event: %w", err)
	}
	return &ev, nil
}
