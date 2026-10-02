package store

import (
	"context"
	"errors"
	"fmt"
)

var ErrProofPayloadTooLarge = errors.New("store: proof event payload exceeds limit")

// SnapshotDiffItem is the compact representation used by snapshot comparisons.
// It intentionally never loads canonical_json.
type SnapshotDiffItem struct {
	ExternalID string
	HashA      []byte
	HashB      []byte
}

// SnapshotDiff holds independently paginated change categories and their totals.
type SnapshotDiff struct {
	Added, Removed, Changed                []SnapshotDiffItem
	AddedTotal, RemovedTotal, ChangedTotal int
}

// SnapshotHashLeaf is the minimal material required to rebuild a Merkle tree.
type SnapshotHashLeaf struct {
	ContentHash []byte
}

// SnapshotProofTarget loads a single proof payload only after its size was checked.
type SnapshotProofTarget struct {
	ID            int64
	ExternalID    string
	ContentHash   []byte
	CanonicalJSON []byte
}

// CompareSnapshots computes the difference in Postgres using only identifiers
// and hashes. Every category receives the same explicit page window.
func (s *Store) CompareSnapshots(ctx context.Context, snapshotAID, snapshotBID int64, limit, offset int) (SnapshotDiff, error) {
	if limit <= 0 || limit > 200 {
		return SnapshotDiff{}, fmt.Errorf("store: diff limit must be between 1 and 200")
	}
	if offset < 0 {
		return SnapshotDiff{}, fmt.Errorf("store: diff offset must not be negative")
	}
	var out SnapshotDiff
	if err := s.pool.QueryRow(ctx, `
		WITH a AS (SELECT external_id, content_hash FROM events WHERE snapshot_id = $1),
		     b AS (SELECT external_id, content_hash FROM events WHERE snapshot_id = $2)
		SELECT
			COUNT(*) FILTER (WHERE a.external_id IS NULL),
			COUNT(*) FILTER (WHERE b.external_id IS NULL),
			COUNT(*) FILTER (WHERE a.external_id IS NOT NULL AND b.external_id IS NOT NULL AND a.content_hash <> b.content_hash)
		FROM a FULL OUTER JOIN b USING (external_id)
	`, snapshotAID, snapshotBID).Scan(&out.AddedTotal, &out.RemovedTotal, &out.ChangedTotal); err != nil {
		return SnapshotDiff{}, fmt.Errorf("store: count snapshot diff: %w", err)
	}

	var err error
	if out.Added, err = s.listSnapshotDiffCategory(ctx, snapshotAID, snapshotBID, "added", limit, offset); err != nil {
		return SnapshotDiff{}, err
	}
	if out.Removed, err = s.listSnapshotDiffCategory(ctx, snapshotAID, snapshotBID, "removed", limit, offset); err != nil {
		return SnapshotDiff{}, err
	}
	if out.Changed, err = s.listSnapshotDiffCategory(ctx, snapshotAID, snapshotBID, "changed", limit, offset); err != nil {
		return SnapshotDiff{}, err
	}
	return out, nil
}

// ListSnapshotHashLeaves reads only ordered IDs and hashes. It requests one
// extra row so a caller never silently builds a truncated Merkle tree.
func (s *Store) ListSnapshotHashLeaves(ctx context.Context, snapshotID int64, cap int) ([]SnapshotHashLeaf, error) {
	if cap <= 0 {
		return nil, fmt.Errorf("store: proof leaf cap must be positive")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT content_hash
		FROM events WHERE snapshot_id = $1
		ORDER BY id ASC LIMIT $2
	`, snapshotID, cap+1)
	if err != nil {
		return nil, fmt.Errorf("store: list proof leaves: %w", err)
	}
	defer rows.Close()
	leaves := make([]SnapshotHashLeaf, 0, min(cap, 1024))
	for rows.Next() {
		var leaf SnapshotHashLeaf
		if err := rows.Scan(&leaf.ContentHash); err != nil {
			return nil, fmt.Errorf("store: scan proof leaf: %w", err)
		}
		leaves = append(leaves, leaf)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(leaves) > cap {
		return nil, fmt.Errorf("store: snapshot %d exceeds proof leaf cap %d", snapshotID, cap)
	}
	return leaves, nil
}

// GetSnapshotProofTarget bounds the JSON payload before loading it.
func (s *Store) GetSnapshotProofTarget(ctx context.Context, snapshotID int64, externalID string, maxPayloadBytes int) (*SnapshotProofTarget, error) {
	var target SnapshotProofTarget
	var payloadBytes int
	err := s.pool.QueryRow(ctx, `SELECT id, external_id, content_hash, octet_length(canonical_json::text) FROM events WHERE snapshot_id = $1 AND external_id = $2`, snapshotID, externalID).Scan(&target.ID, &target.ExternalID, &target.ContentHash, &payloadBytes)
	if err != nil {
		return nil, err
	}
	if payloadBytes > maxPayloadBytes {
		return nil, ErrProofPayloadTooLarge
	}
	if err := s.pool.QueryRow(ctx, `SELECT canonical_json FROM events WHERE id = $1`, target.ID).Scan(&target.CanonicalJSON); err != nil {
		return nil, fmt.Errorf("store: load proof event payload: %w", err)
	}
	return &target, nil
}

// SnapshotProofIndex returns the ordered Merkle index without retaining every
// event identifier in memory.
func (s *Store) SnapshotProofIndex(ctx context.Context, snapshotID, eventID int64) (int, error) {
	var index int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM events WHERE snapshot_id = $1 AND id < $2`, snapshotID, eventID).Scan(&index); err != nil {
		return 0, fmt.Errorf("store: proof index: %w", err)
	}
	return index, nil
}

func (s *Store) listSnapshotDiffCategory(ctx context.Context, snapshotAID, snapshotBID int64, category string, limit, offset int) ([]SnapshotDiffItem, error) {
	var query string
	switch category {
	case "added":
		query = `SELECT b.external_id, NULL::bytea, b.content_hash FROM events b WHERE b.snapshot_id = $2 AND NOT EXISTS (SELECT 1 FROM events a WHERE a.snapshot_id = $1 AND a.external_id = b.external_id) ORDER BY b.external_id LIMIT $3 OFFSET $4`
	case "removed":
		query = `SELECT a.external_id, a.content_hash, NULL::bytea FROM events a WHERE a.snapshot_id = $1 AND NOT EXISTS (SELECT 1 FROM events b WHERE b.snapshot_id = $2 AND b.external_id = a.external_id) ORDER BY a.external_id LIMIT $3 OFFSET $4`
	case "changed":
		query = `SELECT a.external_id, a.content_hash, b.content_hash FROM events a JOIN events b ON b.snapshot_id = $2 AND b.external_id = a.external_id WHERE a.snapshot_id = $1 AND a.content_hash <> b.content_hash ORDER BY a.external_id LIMIT $3 OFFSET $4`
	default:
		return nil, fmt.Errorf("store: unsupported snapshot diff category %q", category)
	}
	rows, err := s.pool.Query(ctx, query, snapshotAID, snapshotBID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("store: list %s snapshot diff: %w", category, err)
	}
	defer rows.Close()
	out := make([]SnapshotDiffItem, 0, limit)
	for rows.Next() {
		var item SnapshotDiffItem
		if err := rows.Scan(&item.ExternalID, &item.HashA, &item.HashB); err != nil {
			return nil, fmt.Errorf("store: scan %s snapshot diff: %w", category, err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
