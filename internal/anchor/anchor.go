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
	pending, err := w.store.ListSnapshotsPendingMerkle(ctx, 20)
	if err != nil {
		return fmt.Errorf("anchor: list pending merkle: %w", err)
	}
	for _, snap := range pending {
		events, err := w.store.ListEventsBySnapshot(ctx, snap.ID)
		if err != nil {
			w.logger.Error().Err(err).Int64("snapshot_id", snap.ID).Msg("anchor: load events failed")
			continue
		}
		if len(events) == 0 {
			w.logger.Warn().Int64("snapshot_id", snap.ID).Msg("anchor: snapshot has no events, skipping")
			continue
		}
		leaves := make([][merkle.HashSize]byte, len(events))
		for i, ev := range events {
			if len(ev.ContentHash) != merkle.HashSize {
				return fmt.Errorf("anchor: snapshot %d event %d: content_hash wrong length", snap.ID, ev.ID)
			}
			var h [merkle.HashSize]byte
			copy(h[:], ev.ContentHash)
			leaves[i] = h
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
		newReceipt, err := w.otsClient.Upgrade(ctx, a.CalendarURL, snap.MerkleRoot)
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
		if err := w.store.MarkAnchorUpgraded(ctx, a.ID, newReceipt, nil); err != nil {
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
