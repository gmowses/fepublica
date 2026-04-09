// Package drift runs the diff detector: it finds pairs of consecutive snapshots
// of the same source that have not been compared yet, computes the per-record
// difference, and persists a diff_run plus one change_event per affected record.
//
// This is the "O1" component of the Observatório program. See
// docs/superpowers/specs/2026-04-09-observatorio-design.md for rationale.
package drift

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/gmowses/fepublica/internal/severity"
	"github.com/gmowses/fepublica/internal/store"
)

// Detector owns the pending-diff queue and runs batches against the store.
type Detector struct {
	store      *store.Store
	classifier *severity.Classifier
	logger     zerolog.Logger
}

// New builds a Detector with the default severity rule set.
func New(s *store.Store, logger zerolog.Logger) *Detector {
	return &Detector{
		store:      s,
		classifier: severity.New(severity.DefaultRules()...),
		logger:     logger,
	}
}

// RunOnce scans for pending snapshot pairs and materializes diffs for up to
// `batchSize` of them. Returns the number of pairs processed.
func (d *Detector) RunOnce(ctx context.Context, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = 20
	}
	candidates, err := d.store.ListSnapshotPairsPendingDiff(ctx, batchSize)
	if err != nil {
		return 0, fmt.Errorf("drift: list pending pairs: %w", err)
	}
	if len(candidates) == 0 {
		return 0, nil
	}
	d.logger.Info().Int("pairs", len(candidates)).Msg("drift: starting batch")

	processed := 0
	for _, c := range candidates {
		if err := ctx.Err(); err != nil {
			return processed, err
		}
		if err := d.processPair(ctx, c); err != nil {
			d.logger.Error().
				Err(err).
				Str("source", c.SourceID).
				Int64("a", c.SnapshotAID).
				Int64("b", c.SnapshotBID).
				Msg("drift: pair failed")
			continue
		}
		processed++
	}
	return processed, nil
}

// processPair runs a single diff and persists the result.
func (d *Detector) processPair(ctx context.Context, c store.DiffRunCandidate) error {
	start := time.Now()

	eventsA, err := d.store.ListEventsBySnapshot(ctx, c.SnapshotAID)
	if err != nil {
		return fmt.Errorf("load events a: %w", err)
	}
	eventsB, err := d.store.ListEventsBySnapshot(ctx, c.SnapshotBID)
	if err != nil {
		return fmt.Errorf("load events b: %w", err)
	}

	mapA := make(map[string]store.Event, len(eventsA))
	for _, e := range eventsA {
		mapA[e.ExternalID] = e
	}
	mapB := make(map[string]store.Event, len(eventsB))
	for _, e := range eventsB {
		mapB[e.ExternalID] = e
	}

	var changes []store.InsertChangeEventParams
	for id, eb := range mapB {
		ea, existed := mapA[id]
		if !existed {
			changes = append(changes, store.InsertChangeEventParams{
				SourceID:     c.SourceID,
				ExternalID:   id,
				ChangeType:   "added",
				ContentHashA: nil,
				ContentHashB: eb.ContentHash,
			})
			continue
		}
		if !bytes.Equal(ea.ContentHash, eb.ContentHash) {
			changes = append(changes, store.InsertChangeEventParams{
				SourceID:     c.SourceID,
				ExternalID:   id,
				ChangeType:   "modified",
				ContentHashA: ea.ContentHash,
				ContentHashB: eb.ContentHash,
			})
		}
	}
	for id, ea := range mapA {
		if _, ok := mapB[id]; !ok {
			changes = append(changes, store.InsertChangeEventParams{
				SourceID:     c.SourceID,
				ExternalID:   id,
				ChangeType:   "removed",
				ContentHashA: ea.ContentHash,
				ContentHashB: nil,
			})
		}
	}

	// Classify severity per event before insert.
	added, removed, modified := 0, 0, 0
	for i := range changes {
		switch changes[i].ChangeType {
		case "added":
			added++
		case "removed":
			removed++
		case "modified":
			modified++
		}
		fakeEv := &store.ChangeEvent{
			SourceID:   changes[i].SourceID,
			ExternalID: changes[i].ExternalID,
			ChangeType: changes[i].ChangeType,
		}
		level, _, _ := d.classifier.Classify(fakeEv, severity.RuleContext{
			Store: d.store,
			Ctx:   ctx,
		})
		changes[i].Severity = level
	}

	runID, err := d.store.CreateDiffRunWithChanges(ctx, store.CreateDiffRunParams{
		SourceID:      c.SourceID,
		SnapshotAID:   c.SnapshotAID,
		SnapshotBID:   c.SnapshotBID,
		AddedCount:    added,
		RemovedCount:  removed,
		ModifiedCount: modified,
		DurationMS:    int(time.Since(start).Milliseconds()),
	}, changes)
	if err != nil {
		return fmt.Errorf("persist diff_run: %w", err)
	}

	d.logger.Info().
		Str("source", c.SourceID).
		Int64("a", c.SnapshotAID).
		Int64("b", c.SnapshotBID).
		Int64("diff_run_id", runID).
		Int("added", added).
		Int("removed", removed).
		Int("modified", modified).
		Dur("elapsed", time.Since(start)).
		Msg("drift: pair processed")
	return nil
}

// Loop runs RunOnce every interval until the context is cancelled.
func (d *Detector) Loop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	_, _ = d.RunOnce(ctx, 0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, _ = d.RunOnce(ctx, 0)
		}
	}
}
