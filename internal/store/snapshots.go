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

// CreateSnapshotWithCounts creates a collecting snapshot with the expected
// final counts. Call CompleteSnapshot only after all event batches commit.
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
			record_count, bytes_size, notes, lifecycle_status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'collecting')
		RETURNING id
	`, p.SourceID, p.CollectedAt, nullIfEmpty(p.APIVersion), p.CollectorVersion,
		recordCount, bytesSize, nullIfEmpty(notes)).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store: create snapshot with counts: %w", err)
	}
	return id, nil
}

// CompleteSnapshot seals a collecting snapshot after checking, while holding
// its row lock, that every expected event has committed.
func (s *Store) CompleteSnapshot(ctx context.Context, id int64) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("store: begin complete snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var expected int
	var status string
	var root []byte
	err = tx.QueryRow(ctx, `
		SELECT record_count, lifecycle_status, merkle_root
		FROM snapshots WHERE id = $1 FOR UPDATE
	`, id).Scan(&expected, &status, &root)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store: snapshot %d not found", id)
		}
		return fmt.Errorf("store: lock snapshot %d: %w", id, err)
	}
	if status != "collecting" || root != nil {
		return fmt.Errorf("store: snapshot %d cannot be completed from %q", id, status)
	}

	var actual int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM events WHERE snapshot_id = $1`, id).Scan(&actual); err != nil {
		return fmt.Errorf("store: count snapshot %d events: %w", id, err)
	}
	if actual != expected {
		return fmt.Errorf("store: snapshot %d has %d events; expected %d", id, actual, expected)
	}
	if _, err := tx.Exec(ctx, `UPDATE snapshots SET lifecycle_status = 'complete' WHERE id = $1`, id); err != nil {
		return fmt.Errorf("store: complete snapshot %d: %w", id, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit completed snapshot %d: %w", id, err)
	}
	return nil
}

// FailSnapshot closes a collecting snapshot after a collection error. Its
// already committed events remain immutable evidence of the failed run.
func (s *Store) FailSnapshot(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE snapshots
		SET lifecycle_status = 'failed'
		WHERE id = $1 AND lifecycle_status = 'collecting'
	`, id)
	if err != nil {
		return fmt.Errorf("store: fail snapshot %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("store: snapshot %d is not collecting or does not exist", id)
	}
	return nil
}

// SetSnapshotMerkleRoot writes the root hash once for a complete snapshot. It
// repeats the event count check under the snapshot row lock as a final guard.
func (s *Store) SetSnapshotMerkleRoot(ctx context.Context, id int64, root []byte) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("store: begin set merkle root: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var expected int
	var status string
	var existing []byte
	err = tx.QueryRow(ctx, `
		SELECT record_count, lifecycle_status, merkle_root
		FROM snapshots WHERE id = $1 FOR UPDATE
	`, id).Scan(&expected, &status, &existing)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store: snapshot %d not found", id)
		}
		return fmt.Errorf("store: lock snapshot %d: %w", id, err)
	}
	if status != "complete" || existing != nil {
		return fmt.Errorf("store: snapshot %d is not complete or already has a merkle root", id)
	}
	var actual int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM events WHERE snapshot_id = $1`, id).Scan(&actual); err != nil {
		return fmt.Errorf("store: count snapshot %d events: %w", id, err)
	}
	if actual != expected {
		return fmt.Errorf("store: snapshot %d has %d events; expected %d", id, actual, expected)
	}
	if _, err := tx.Exec(ctx, `UPDATE snapshots SET merkle_root = $1, merkle_computed_at = NOW() WHERE id = $2`, root, id); err != nil {
		return fmt.Errorf("store: set merkle root: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit merkle root: %w", err)
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
		       merkle_root, merkle_computed_at, collector_version, notes, lifecycle_status
		FROM snapshots WHERE id = $1
	`, id).Scan(
		&snap.ID, &snap.SourceID, &snap.CollectedAt, &apiVersion,
		&snap.RecordCount, &snap.BytesSize, &snap.MerkleRoot, &merkleComputed,
		&snap.CollectorVersion, &notes, &snap.LifecycleStatus,
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
		       merkle_root, merkle_computed_at, collector_version, COALESCE(notes, ''), lifecycle_status
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
			&snap.CollectorVersion, &snap.Notes, &snap.LifecycleStatus,
		); err != nil {
			return nil, fmt.Errorf("store: scan snapshot: %w", err)
		}
		snap.MerkleComputedAt = merkleComputed
		out = append(out, snap)
	}
	return out, rows.Err()
}

// ListSnapshotsPendingMerkle returns complete snapshots eligible for a Merkle
// root, oldest first, up to limit. maxRecords prevents oversized earlier
// snapshots from starving eligible work.
func (s *Store) ListSnapshotsPendingMerkle(ctx context.Context, limit, maxRecords int) ([]Snapshot, error) {
	if limit <= 0 {
		limit = 10
	}
	if maxRecords <= 0 {
		return nil, fmt.Errorf("store: maximum record count must be positive")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, source_id, collected_at, COALESCE(api_version, ''), record_count, bytes_size,
		       merkle_root, merkle_computed_at, collector_version, COALESCE(notes, ''), lifecycle_status
		FROM snapshots
		WHERE merkle_root IS NULL AND record_count > 0 AND record_count <= $1
		  AND lifecycle_status = 'complete'
		ORDER BY collected_at ASC
		LIMIT $2
	`, maxRecords, limit)
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
			&snap.CollectorVersion, &snap.Notes, &snap.LifecycleStatus,
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
