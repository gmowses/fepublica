package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CreateSnapshotParams are the inputs to create a new snapshot.
type CreateSnapshotParams struct {
	SourceID         string
	CollectedAt      time.Time
	APIVersion       string
	CollectorVersion string
}

// CreateSnapshotWithCounts is the intended entry point: create a snapshot row
// with its final counts already known. Snapshots are immutable after insertion
// (except for the merkle_root set-once and notes), so the collector must
// assemble all events in memory before calling this.
func (s *Store) CreateSnapshotWithCounts(
	ctx context.Context,
	p CreateSnapshotParams,
	recordCount int,
	bytesSize int64,
	notes string,
) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO snapshots (
			source_id, collected_at, api_version, collector_version,
			record_count, bytes_size, notes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, p.SourceID, p.CollectedAt, nullIfEmpty(p.APIVersion), p.CollectorVersion,
		recordCount, bytesSize, nullIfEmpty(notes)).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store: create snapshot with counts: %w", err)
	}
	return id, nil
}

// SetSnapshotMerkleRoot writes the root hash once. It fails if already set.
func (s *Store) SetSnapshotMerkleRoot(ctx context.Context, id int64, root []byte) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE snapshots
		SET merkle_root = $1, merkle_computed_at = NOW()
		WHERE id = $2 AND merkle_root IS NULL
	`, root, id)
	if err != nil {
		return fmt.Errorf("store: set merkle root: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: snapshot %d already has a merkle root or does not exist", id)
	}
	return nil
}

// GetSnapshot returns a single snapshot by id.
func (s *Store) GetSnapshot(ctx context.Context, id int64) (*Snapshot, error) {
	var snap Snapshot
	var apiVersion, notes *string
	var merkleComputed *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, source_id, collected_at, api_version, record_count, bytes_size,
		       merkle_root, merkle_computed_at, collector_version, notes
		FROM snapshots WHERE id = $1
	`, id).Scan(
		&snap.ID, &snap.SourceID, &snap.CollectedAt, &apiVersion,
		&snap.RecordCount, &snap.BytesSize, &snap.MerkleRoot, &merkleComputed,
		&snap.CollectorVersion, &notes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("store: snapshot %d not found", id)
		}
		return nil, fmt.Errorf("store: get snapshot: %w", err)
	}
	if apiVersion != nil {
		snap.APIVersion = *apiVersion
	}
	if notes != nil {
		snap.Notes = *notes
	}
	snap.MerkleComputedAt = merkleComputed
	return &snap, nil
}

// ListSnapshots returns snapshots for a given source, most recent first.
func (s *Store) ListSnapshots(ctx context.Context, sourceID string, limit int) ([]Snapshot, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, source_id, collected_at, COALESCE(api_version, ''), record_count, bytes_size,
		       merkle_root, merkle_computed_at, collector_version, COALESCE(notes, '')
		FROM snapshots
		WHERE ($1 = '' OR source_id = $1)
		ORDER BY collected_at DESC
		LIMIT $2
	`, sourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list snapshots: %w", err)
	}
	defer rows.Close()

	var out []Snapshot
	for rows.Next() {
		var snap Snapshot
		var merkleComputed *time.Time
		if err := rows.Scan(
			&snap.ID, &snap.SourceID, &snap.CollectedAt, &snap.APIVersion,
			&snap.RecordCount, &snap.BytesSize, &snap.MerkleRoot, &merkleComputed,
			&snap.CollectorVersion, &snap.Notes,
		); err != nil {
			return nil, fmt.Errorf("store: scan snapshot: %w", err)
		}
		snap.MerkleComputedAt = merkleComputed
		out = append(out, snap)
	}
	return out, rows.Err()
}

// ListSnapshotsPendingMerkle returns snapshots that have no merkle root yet,
// oldest first, up to limit.
func (s *Store) ListSnapshotsPendingMerkle(ctx context.Context, limit int) ([]Snapshot, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, source_id, collected_at, COALESCE(api_version, ''), record_count, bytes_size,
		       merkle_root, merkle_computed_at, collector_version, COALESCE(notes, '')
		FROM snapshots
		WHERE merkle_root IS NULL AND record_count > 0
		ORDER BY collected_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list pending merkle snapshots: %w", err)
	}
	defer rows.Close()

	var out []Snapshot
	for rows.Next() {
		var snap Snapshot
		var merkleComputed *time.Time
		if err := rows.Scan(
			&snap.ID, &snap.SourceID, &snap.CollectedAt, &snap.APIVersion,
			&snap.RecordCount, &snap.BytesSize, &snap.MerkleRoot, &merkleComputed,
			&snap.CollectorVersion, &snap.Notes,
		); err != nil {
			return nil, fmt.Errorf("store: scan pending merkle snapshot: %w", err)
		}
		snap.MerkleComputedAt = merkleComputed
		out = append(out, snap)
	}
	return out, rows.Err()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
