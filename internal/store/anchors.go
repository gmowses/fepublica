package store

import (
	"context"
	"fmt"
	"time"
)

// InsertAnchorParams holds the initial anchor submission.
type InsertAnchorParams struct {
	SnapshotID  int64
	CalendarURL string
	Receipt     []byte
}

// InsertAnchor records a newly submitted anchor.
func (s *Store) InsertAnchor(ctx context.Context, p InsertAnchorParams) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO anchors (snapshot_id, calendar_url, submitted_at, receipt)
		VALUES ($1, $2, NOW(), $3)
		ON CONFLICT (snapshot_id, calendar_url) DO NOTHING
		RETURNING id
	`, p.SnapshotID, p.CalendarURL, p.Receipt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store: insert anchor: %w", err)
	}
	return id, nil
}

// ListAnchorsForSnapshot returns all anchors for a snapshot.
func (s *Store) ListAnchorsForSnapshot(ctx context.Context, snapshotID int64) ([]Anchor, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, snapshot_id, calendar_url, submitted_at, receipt, upgraded, upgraded_at, block_height
		FROM anchors
		WHERE snapshot_id = $1
		ORDER BY submitted_at ASC
	`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("store: list anchors: %w", err)
	}
	defer rows.Close()

	var out []Anchor
	for rows.Next() {
		var a Anchor
		var upgradedAt *time.Time
		var blockHeight *int
		if err := rows.Scan(
			&a.ID, &a.SnapshotID, &a.CalendarURL, &a.SubmittedAt,
			&a.Receipt, &a.Upgraded, &upgradedAt, &blockHeight,
		); err != nil {
			return nil, fmt.Errorf("store: scan anchor: %w", err)
		}
		a.UpgradedAt = upgradedAt
		a.BlockHeight = blockHeight
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListPendingUpgradeAnchors returns anchors that have not been upgraded yet, oldest first.
func (s *Store) ListPendingUpgradeAnchors(ctx context.Context, limit int) ([]Anchor, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, snapshot_id, calendar_url, submitted_at, receipt, upgraded, upgraded_at, block_height
		FROM anchors
		WHERE upgraded = FALSE
		ORDER BY submitted_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list pending upgrade anchors: %w", err)
	}
	defer rows.Close()

	var out []Anchor
	for rows.Next() {
		var a Anchor
		var upgradedAt *time.Time
		var blockHeight *int
		if err := rows.Scan(
			&a.ID, &a.SnapshotID, &a.CalendarURL, &a.SubmittedAt,
			&a.Receipt, &a.Upgraded, &upgradedAt, &blockHeight,
		); err != nil {
			return nil, fmt.Errorf("store: scan anchor: %w", err)
		}
		a.UpgradedAt = upgradedAt
		a.BlockHeight = blockHeight
		out = append(out, a)
	}
	return out, rows.Err()
}

// MarkAnchorUpgraded replaces the receipt with the upgraded version and sets metadata.
func (s *Store) MarkAnchorUpgraded(ctx context.Context, anchorID int64, newReceipt []byte, blockHeight *int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE anchors
		SET receipt = $1,
		    upgraded = TRUE,
		    upgraded_at = NOW(),
		    block_height = $2
		WHERE id = $3 AND upgraded = FALSE
	`, newReceipt, blockHeight, anchorID)
	if err != nil {
		return fmt.Errorf("store: mark anchor upgraded: %w", err)
	}
	return nil
}

// SnapshotsMissingAnchor returns snapshots with merkle_root set but no anchors in the given calendar.
func (s *Store) SnapshotsMissingAnchor(ctx context.Context, calendarURL string, limit int) ([]Snapshot, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, s.source_id, s.collected_at, COALESCE(s.api_version, ''),
		       s.record_count, s.bytes_size, s.merkle_root, s.merkle_computed_at,
		       s.collector_version, COALESCE(s.notes, '')
		FROM snapshots s
		WHERE s.merkle_root IS NOT NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM anchors a
		      WHERE a.snapshot_id = s.id AND a.calendar_url = $1
		  )
		ORDER BY s.collected_at ASC
		LIMIT $2
	`, calendarURL, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list snapshots missing anchor: %w", err)
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
