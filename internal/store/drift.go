package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// DiffRun is an audit-trail row: "at time T we ran a diff between snapshot A
// and snapshot B of source S and found counts X/Y/Z". Written even when no
// changes are found, so we can tell "we checked and there was nothing" apart
// from "we never checked".
type DiffRun struct {
	ID            int64
	SourceID      string
	SnapshotAID   int64
	SnapshotBID   int64
	AddedCount    int
	RemovedCount  int
	ModifiedCount int
	RanAt         time.Time
	DurationMS    int
}

// ChangeEvent is a single detected change (add/remove/modify) between two
// snapshots of the same source. Joined to a DiffRun via diff_run_id.
type ChangeEvent struct {
	ID                int64
	DiffRunID         int64
	SourceID          string
	EnteID            *string
	ExternalID        string
	ChangeType        string
	ContentHashA      []byte
	ContentHashB      []byte
	DetectedAt        time.Time
	Severity          string
	PublishedRSS      bool
	PublishedTelegram bool
	PublishedMastodon bool
	PublishedWebhook  bool
	PublishedEmail    bool
}

// CreateDiffRunParams are the counts a diff run needs to be persisted.
type CreateDiffRunParams struct {
	SourceID      string
	SnapshotAID   int64
	SnapshotBID   int64
	AddedCount    int
	RemovedCount  int
	ModifiedCount int
	DurationMS    int
}

// InsertChangeEventParams holds a single change event row ready for batch insert.
type InsertChangeEventParams struct {
	SourceID     string
	ExternalID   string
	ChangeType   string // "added" | "removed" | "modified"
	ContentHashA []byte
	ContentHashB []byte
	Severity     string
}

// CreateDiffRunWithChanges inserts a diff_run row and all its change_events in
// a single transaction. Returns the new diff_run id.
func (s *Store) CreateDiffRunWithChanges(
	ctx context.Context,
	run CreateDiffRunParams,
	changes []InsertChangeEventParams,
) (int64, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("store: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var runID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO diff_runs (
			source_id, snapshot_a_id, snapshot_b_id,
			added_count, removed_count, modified_count,
			ran_at, duration_ms
		) VALUES ($1, $2, $3, $4, $5, $6, NOW(), $7)
		ON CONFLICT (snapshot_a_id, snapshot_b_id) DO NOTHING
		RETURNING id
	`, run.SourceID, run.SnapshotAID, run.SnapshotBID,
		run.AddedCount, run.RemovedCount, run.ModifiedCount,
		run.DurationMS).Scan(&runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("store: diff run already exists for (%d,%d)",
				run.SnapshotAID, run.SnapshotBID)
		}
		return 0, fmt.Errorf("store: insert diff_run: %w", err)
	}

	if len(changes) > 0 {
		batch := &pgx.Batch{}
		for _, c := range changes {
			severity := c.Severity
			if severity == "" {
				severity = "info"
			}
			batch.Queue(`
				INSERT INTO change_events (
					diff_run_id, source_id, external_id, change_type,
					content_hash_a, content_hash_b, detected_at, severity
				) VALUES ($1, $2, $3, $4, $5, $6, NOW(), $7)
			`, runID, c.SourceID, c.ExternalID, c.ChangeType,
				c.ContentHashA, c.ContentHashB, severity)
		}
		br := tx.SendBatch(ctx, batch)
		for i := 0; i < len(changes); i++ {
			if _, err := br.Exec(); err != nil {
				_ = br.Close()
				return 0, fmt.Errorf("store: insert change_event %d: %w", i, err)
			}
		}
		if err := br.Close(); err != nil {
			return 0, fmt.Errorf("store: close change_events batch: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("store: commit diff run: %w", err)
	}
	return runID, nil
}

// ListSnapshotPairsPendingDiff returns pairs of consecutive snapshots of the
// same source where no diff_run row exists yet, up to limit. The pairs are
// (older, newer) and both sides must have a computed merkle root.
func (s *Store) ListSnapshotPairsPendingDiff(
	ctx context.Context,
	limit int,
) ([]DiffRunCandidate, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		WITH ordered AS (
			SELECT id, source_id, collected_at,
			       LAG(id) OVER (PARTITION BY source_id ORDER BY collected_at ASC) AS prev_id
			FROM snapshots
			WHERE merkle_root IS NOT NULL
		)
		SELECT prev_id AS a, id AS b, source_id
		FROM ordered
		WHERE prev_id IS NOT NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM diff_runs dr
		      WHERE dr.snapshot_a_id = ordered.prev_id
		        AND dr.snapshot_b_id = ordered.id
		  )
		ORDER BY b ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list pending diff pairs: %w", err)
	}
	defer rows.Close()

	var out []DiffRunCandidate
	for rows.Next() {
		var c DiffRunCandidate
		if err := rows.Scan(&c.SnapshotAID, &c.SnapshotBID, &c.SourceID); err != nil {
			return nil, fmt.Errorf("store: scan pending diff pair: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DiffRunCandidate is a pair of consecutive snapshots of the same source that
// have not yet been diffed.
type DiffRunCandidate struct {
	SourceID    string
	SnapshotAID int64
	SnapshotBID int64
}

// ListDiffRuns returns diff_runs filtered and paginated.
func (s *Store) ListDiffRuns(
	ctx context.Context,
	sourceID string,
	limit int,
) ([]DiffRun, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, source_id, snapshot_a_id, snapshot_b_id,
		       added_count, removed_count, modified_count,
		       ran_at, duration_ms
		FROM diff_runs
		WHERE ($1 = '' OR source_id = $1)
		ORDER BY ran_at DESC
		LIMIT $2
	`, sourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list diff_runs: %w", err)
	}
	defer rows.Close()

	var out []DiffRun
	for rows.Next() {
		var dr DiffRun
		if err := rows.Scan(
			&dr.ID, &dr.SourceID, &dr.SnapshotAID, &dr.SnapshotBID,
			&dr.AddedCount, &dr.RemovedCount, &dr.ModifiedCount,
			&dr.RanAt, &dr.DurationMS,
		); err != nil {
			return nil, fmt.Errorf("store: scan diff_run: %w", err)
		}
		out = append(out, dr)
	}
	return out, rows.Err()
}

// GetDiffRun returns a single diff_run by id.
func (s *Store) GetDiffRun(ctx context.Context, id int64) (*DiffRun, error) {
	var dr DiffRun
	err := s.pool.QueryRow(ctx, `
		SELECT id, source_id, snapshot_a_id, snapshot_b_id,
		       added_count, removed_count, modified_count,
		       ran_at, duration_ms
		FROM diff_runs WHERE id = $1
	`, id).Scan(
		&dr.ID, &dr.SourceID, &dr.SnapshotAID, &dr.SnapshotBID,
		&dr.AddedCount, &dr.RemovedCount, &dr.ModifiedCount,
		&dr.RanAt, &dr.DurationMS,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("store: diff_run %d not found", id)
		}
		return nil, fmt.Errorf("store: get diff_run: %w", err)
	}
	return &dr, nil
}

// ListChangeEventsParams filters the change_events listing.
type ListChangeEventsParams struct {
	SourceID   string
	EnteID     string
	Severity   string // "" | "info" | "warn" | "alert"
	ChangeType string // "" | "added" | "removed" | "modified"
	Since      *time.Time
	Limit      int
	Offset     int
}

// ListChangeEvents returns change_events filtered and paginated, plus the
// total count matching the filter (for UI pagination).
func (s *Store) ListChangeEvents(
	ctx context.Context,
	p ListChangeEventsParams,
) ([]ChangeEvent, int, error) {
	if p.Limit <= 0 || p.Limit > 500 {
		p.Limit = 50
	}

	conds := []string{"1=1"}
	args := []any{}
	if p.SourceID != "" {
		args = append(args, p.SourceID)
		conds = append(conds, fmt.Sprintf("source_id = $%d", len(args)))
	}
	if p.EnteID != "" {
		args = append(args, p.EnteID)
		conds = append(conds, fmt.Sprintf("ente_id = $%d", len(args)))
	}
	if p.Severity != "" {
		args = append(args, p.Severity)
		conds = append(conds, fmt.Sprintf("severity = $%d", len(args)))
	}
	if p.ChangeType != "" {
		args = append(args, p.ChangeType)
		conds = append(conds, fmt.Sprintf("change_type = $%d", len(args)))
	}
	if p.Since != nil {
		args = append(args, *p.Since)
		conds = append(conds, fmt.Sprintf("detected_at >= $%d", len(args)))
	}
	where := strings.Join(conds, " AND ")

	// Count
	var total int
	countQ := "SELECT COUNT(*) FROM change_events WHERE " + where
	if err := s.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("store: count change_events: %w", err)
	}

	args = append(args, p.Limit, p.Offset)
	listQ := fmt.Sprintf(`
		SELECT id, diff_run_id, source_id, ente_id, external_id, change_type,
		       content_hash_a, content_hash_b, detected_at, severity,
		       published_rss, published_telegram, published_mastodon,
		       published_webhook, published_email
		FROM change_events
		WHERE %s
		ORDER BY detected_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("store: list change_events: %w", err)
	}
	defer rows.Close()

	var out []ChangeEvent
	for rows.Next() {
		var ce ChangeEvent
		if err := rows.Scan(
			&ce.ID, &ce.DiffRunID, &ce.SourceID, &ce.EnteID, &ce.ExternalID, &ce.ChangeType,
			&ce.ContentHashA, &ce.ContentHashB, &ce.DetectedAt, &ce.Severity,
			&ce.PublishedRSS, &ce.PublishedTelegram, &ce.PublishedMastodon,
			&ce.PublishedWebhook, &ce.PublishedEmail,
		); err != nil {
			return nil, 0, fmt.Errorf("store: scan change_event: %w", err)
		}
		out = append(out, ce)
	}
	return out, total, rows.Err()
}

// GetChangeEvent returns a single change_event by id.
func (s *Store) GetChangeEvent(ctx context.Context, id int64) (*ChangeEvent, error) {
	var ce ChangeEvent
	err := s.pool.QueryRow(ctx, `
		SELECT id, diff_run_id, source_id, ente_id, external_id, change_type,
		       content_hash_a, content_hash_b, detected_at, severity,
		       published_rss, published_telegram, published_mastodon,
		       published_webhook, published_email
		FROM change_events WHERE id = $1
	`, id).Scan(
		&ce.ID, &ce.DiffRunID, &ce.SourceID, &ce.EnteID, &ce.ExternalID, &ce.ChangeType,
		&ce.ContentHashA, &ce.ContentHashB, &ce.DetectedAt, &ce.Severity,
		&ce.PublishedRSS, &ce.PublishedTelegram, &ce.PublishedMastodon,
		&ce.PublishedWebhook, &ce.PublishedEmail,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("store: change_event %d not found", id)
		}
		return nil, fmt.Errorf("store: get change_event: %w", err)
	}
	return &ce, nil
}

// UpdateChangeEventSeverity updates the severity field (the severity
// classifier can re-run and upgrade events post-hoc).
func (s *Store) UpdateChangeEventSeverity(ctx context.Context, id int64, severity string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE change_events SET severity = $1 WHERE id = $2
	`, severity, id)
	if err != nil {
		return fmt.Errorf("store: update change_event severity: %w", err)
	}
	return nil
}
