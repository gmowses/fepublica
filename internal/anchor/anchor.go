// Package anchor builds Merkle trees for snapshots and submits their roots to
// one or more OpenTimestamps calendars.
package anchor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/gmowses/fepublica/internal/merkle"
	"github.com/gmowses/fepublica/internal/metrics"
	"github.com/gmowses/fepublica/internal/ots"
	"github.com/gmowses/fepublica/internal/store"
)

// Worker processes snapshots that are missing Merkle roots and/or anchors.
type Worker struct {
	store     *store.Store
	otsClient *ots.Client
	calendars []string
	logger    zerolog.Logger
}

const maxMerkleLeaves = 200_000

// New creates an anchor Worker.
func New(s *store.Store, calendars []string, logger zerolog.Logger) *Worker {
	return &Worker{
		store:     s,
		otsClient: ots.NewClient(),
		calendars: calendars,
		logger:    logger,
	}
}

// RunOnce performs a full pass: compute merkle roots for any pending snapshots,
// submit roots to all configured calendars, then attempt to upgrade any
// receipts that haven't been upgraded yet.
func (w *Worker) RunOnce(ctx context.Context) error {
	if err := w.buildPendingRoots(ctx); err != nil {
		w.logger.Error().Err(err).Msg("anchor: build pending roots failed")
	}
	if err := w.submitMissingAnchors(ctx); err != nil {
		w.logger.Error().Err(err).Msg("anchor: submit missing anchors failed")
	}
	if err := w.upgradePendingAnchors(ctx); err != nil {
		w.logger.Error().Err(err).Msg("anchor: upgrade pending anchors failed")
	}
	return nil
}

func (w *Worker) buildPendingRoots(ctx context.Context) error {
	pending, err := w.store.ListSnapshotsPendingMerkle(ctx, 20, maxMerkleLeaves)
	if err != nil {
		return fmt.Errorf("anchor: list pending merkle: %w", err)
	}
	for _, snap := range pending {
		if err := validateMerkleLeafCount(snap.ID, snap.RecordCount, snap.RecordCount); err != nil {
			w.logger.Warn().Err(err).Int64("snapshot_id", snap.ID).Msg("anchor: snapshot skipped")
			continue
		}
		storedLeaves, err := w.store.ListSnapshotHashLeaves(ctx, snap.ID, maxMerkleLeaves)
		if err != nil {
			w.logger.Warn().Err(err).Int64("snapshot_id", snap.ID).Msg("anchor: load snapshot hashes skipped")
			continue
		}
		if err := validateMerkleLeafCount(snap.ID, snap.RecordCount, len(storedLeaves)); err != nil {
			w.logger.Warn().Err(err).Int64("snapshot_id", snap.ID).Msg("anchor: snapshot skipped")
			continue
		}
		leaves, err := merkleLeaves(snap.ID, storedLeaves)
		if err != nil {
			w.logger.Warn().Err(err).Int64("snapshot_id", snap.ID).Msg("anchor: snapshot skipped")
			continue
		}
		tree, err := merkle.Build(leaves)
		if err != nil {
			return fmt.Errorf("anchor: build tree for snapshot %d: %w", snap.ID, err)
		}
		root := tree.Root()
		if err := w.store.SetSnapshotMerkleRoot(ctx, snap.ID, root[:]); err != nil {
			return fmt.Errorf("anchor: set root for snapshot %d: %w", snap.ID, err)
		}
		metrics.AnchorBuildRootsTotal.Inc()
		w.logger.Info().
			Int64("snapshot_id", snap.ID).
			Int("leaves", len(leaves)).
			Str("root", store.HexHash(root[:])).
			Msg("anchor: merkle root computed")
	}
	return nil
}

func validateMerkleLeafCount(snapshotID int64, expected, actual int) error {
	if expected <= 0 {
		return fmt.Errorf("anchor: snapshot %d has non-positive record count %d", snapshotID, expected)
	}
	if expected > maxMerkleLeaves {
		return fmt.Errorf("anchor: snapshot %d has %d records; maximum is %d", snapshotID, expected, maxMerkleLeaves)
	}
	if actual != expected {
		return fmt.Errorf("anchor: snapshot %d has %d hash leaves; expected %d", snapshotID, actual, expected)
	}
	return nil
}

func merkleLeaves(snapshotID int64, storedLeaves []store.SnapshotHashLeaf) ([][merkle.HashSize]byte, error) {
	leaves := make([][merkle.HashSize]byte, len(storedLeaves))
	for i, leaf := range storedLeaves {
		if len(leaf.ContentHash) != merkle.HashSize {
			return nil, fmt.Errorf("anchor: snapshot %d hash leaf %d has length %d; expected %d", snapshotID, i, len(leaf.ContentHash), merkle.HashSize)
		}
		copy(leaves[i][:], leaf.ContentHash)
	}
	return leaves, nil
}

func (w *Worker) submitMissingAnchors(ctx context.Context) error {
	for _, cal := range w.calendars {
		missing, err := w.store.SnapshotsMissingAnchor(ctx, cal, 20)
		if err != nil {
			return fmt.Errorf("anchor: list missing for %s: %w", cal, err)
		}
		for _, snap := range missing {
			if len(snap.MerkleRoot) != merkle.HashSize {
				continue
			}
			receipt, err := w.otsClient.Submit(ctx, cal, snap.MerkleRoot)
			if err != nil {
				metrics.AnchorSubmitTotal.WithLabelValues(cal, "error").Inc()
				w.logger.Error().
					Err(err).
					Int64("snapshot_id", snap.ID).
					Str("calendar", cal).
					Msg("anchor: submit failed")
				continue
			}
			metrics.AnchorSubmitTotal.WithLabelValues(cal, "ok").Inc()
			_, err = w.store.InsertAnchor(ctx, store.InsertAnchorParams{
				SnapshotID:  snap.ID,
				CalendarURL: cal,
				Receipt:     receipt,
			})
			if err != nil {
				w.logger.Error().
					Err(err).
					Int64("snapshot_id", snap.ID).
					Str("calendar", cal).
					Msg("anchor: persist anchor failed")
				continue
			}
			w.logger.Info().
				Int64("snapshot_id", snap.ID).
				Str("calendar", cal).
				Int("receipt_bytes", len(receipt)).
				Msg("anchor: submitted")
		}
	}
	return nil
}

func (w *Worker) upgradePendingAnchors(ctx context.Context) error {
	pending, err := w.store.ListPendingUpgradeAnchors(ctx, 100)
	if err != nil {
		return fmt.Errorf("anchor: list pending upgrades: %w", err)
	}
	for _, a := range pending {
		// Fetch the snapshot to get the merkle root (the digest we need to upgrade).
		snap, err := w.store.GetSnapshot(ctx, a.SnapshotID)
		if err != nil {
			w.logger.Error().Err(err).Int64("anchor_id", a.ID).Msg("anchor: load snapshot for upgrade")
			continue
		}
		if len(snap.MerkleRoot) != merkle.HashSize {
			continue
		}
		newReceipt, blockHeight, err := w.otsClient.Upgrade(ctx, a.CalendarURL, snap.MerkleRoot)
		if err != nil {
			if errors.Is(err, ots.ErrNotReady) {
				metrics.AnchorUpgradeTotal.WithLabelValues(a.CalendarURL, "not_ready").Inc()
				w.logger.Debug().
					Int64("anchor_id", a.ID).
					Str("calendar", a.CalendarURL).
					Msg("anchor: upgrade not ready yet")
				continue
			}
			metrics.AnchorUpgradeTotal.WithLabelValues(a.CalendarURL, "error").Inc()
			w.logger.Error().
				Err(err).
				Int64("anchor_id", a.ID).
				Str("calendar", a.CalendarURL).
				Msg("anchor: upgrade request failed")
			continue
		}
		if err := w.store.MarkAnchorUpgraded(ctx, a.ID, newReceipt, blockHeight); err != nil {
			metrics.AnchorUpgradeTotal.WithLabelValues(a.CalendarURL, "persist_error").Inc()
			w.logger.Error().Err(err).Int64("anchor_id", a.ID).Msg("anchor: mark upgraded failed")
			continue
		}
		metrics.AnchorUpgradeTotal.WithLabelValues(a.CalendarURL, "ok").Inc()
		w.logger.Info().
			Int64("anchor_id", a.ID).
			Str("calendar", a.CalendarURL).
			Msg("anchor: upgraded")
	}
	return nil
}

// Loop runs RunOnce in a loop every interval. Exits on context cancellation.
func (w *Worker) Loop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	_ = w.RunOnce(ctx) // run immediately on startup
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = w.RunOnce(ctx)
		}
	}
}
