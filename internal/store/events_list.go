package store

import (
	"context"
	"fmt"
	"strings"
)

// EventMeta is a lightweight view of an event, without the full canonical_json
// payload. Used for listing and browsing without shipping megabytes over the wire.
type EventMeta struct {
	ID          int64
	SnapshotID  int64
	SourceID    string
	ExternalID  string
	ContentHash []byte
}

// ListEventMetaParams controls ListEventMeta.
type ListEventMetaParams struct {
	SnapshotID int64
	Search     string // optional substring filter on external_id
	Limit      int
	Offset     int
}

// ListEventMeta returns a page of lightweight event metadata for a snapshot.
// Use it to build browse/search UIs without transferring full canonical payloads.
func (s *Store) ListEventMeta(ctx context.Context, p ListEventMetaParams) ([]EventMeta, int, error) {
	if p.Limit <= 0 || p.Limit > 1000 {
		p.Limit = 100
	}
	if p.Offset < 0 {
		p.Offset = 0
	}

	conds := []string{"snapshot_id = $1"}
	args := []any{p.SnapshotID}
	if p.Search != "" {
		conds = append(conds, fmt.Sprintf("external_id ILIKE $%d", len(args)+1))
		args = append(args, "%"+p.Search+"%")
	}

	where := strings.Join(conds, " AND ")

	// Total count
	var total int
	countQ := "SELECT COUNT(*) FROM events WHERE " + where
	if err := s.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("store: count events: %w", err)
	}

	args = append(args, p.Limit, p.Offset)
	listQ := fmt.Sprintf(`
		SELECT id, snapshot_id, source_id, external_id, content_hash
		FROM events
		WHERE %s
		ORDER BY id ASC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: list event meta: %w", err)
	}
	defer rows.Close()

	var out []EventMeta
	for rows.Next() {
		var ev EventMeta
		if err := rows.Scan(&ev.ID, &ev.SnapshotID, &ev.SourceID, &ev.ExternalID, &ev.ContentHash); err != nil {
			return nil, 0, fmt.Errorf("store: scan event meta: %w", err)
		}
		out = append(out, ev)
	}
	return out, total, rows.Err()
}
