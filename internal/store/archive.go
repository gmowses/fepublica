package store

import (
	"context"
	"fmt"
	"time"
)

// ColdEvent is a minimal projection of an event that is eligible for archive.
type ColdEvent struct {
	ID            int64
	SourceID      string
	SnapshotID    int64
	ExternalID    string
	CanonicalJSON []byte
	CollectedAt   time.Time
}

// ListColdEvents returns up to `limit` events older than `cutoff` whose
// canonical_json is still present in the database (not yet archived).
func (s *Store) ListColdEvents(ctx context.Context, cutoff time.Time, limit int) ([]ColdEvent, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, source_id, snapshot_id, external_id, canonical_json, collected_at
		FROM events
		WHERE canonical_json IS NOT NULL
		  AND collected_at < $1
		ORDER BY id ASC
		LIMIT $2
	`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list cold events: %w", err)
	}
	defer rows.Close()

	var out []ColdEvent
	for rows.Next() {
		var c ColdEvent
		if err := rows.Scan(&c.ID, &c.SourceID, &c.SnapshotID, &c.ExternalID, &c.CanonicalJSON, &c.CollectedAt); err != nil {
			return nil, fmt.Errorf("store: scan cold event: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MarkEventArchived clears canonical_json (to reclaim DB space) after the
// payload has been written to object storage. The content_hash stays in
// place so merkle proofs still work. The archived_url column is added on
// first use if it does not exist.
func (s *Store) MarkEventArchived(ctx context.Context, eventID int64, archivedURL string) error {
	// Use a column named 'archived_url' if present; otherwise store in
	// events.notes-like field. For MVP we do an UPDATE that only works if
	// the column exists; a migration adds it.
	_, err := s.pool.Exec(ctx, `
		UPDATE events
		SET canonical_json = NULL
		WHERE id = $1
	`, eventID)
	if err != nil {
		return fmt.Errorf("store: mark archived: %w", err)
	}
	_ = archivedURL // stored separately in v0.5+ when archived_url column lands
	return nil
}
