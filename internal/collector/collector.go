// Package collector orchestrates a single collection run: it fetches records
// from a source, canonicalizes them, hashes them, and persists a snapshot
// plus events to the store.
package collector

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/gmowses/fepublica/internal/canonjson"
	"github.com/gmowses/fepublica/internal/metrics"
	"github.com/gmowses/fepublica/internal/store"
	"github.com/gmowses/fepublica/internal/transparencia"
)

// Fetcher is implemented by each source (ceis, cnep, ...).
type Fetcher func(ctx context.Context, client *transparencia.Client) (*transparencia.FetchResult, error)

// Collector runs a named source against the API and persists the result.
type Collector struct {
	store      *store.Store
	httpClient *transparencia.Client
	logger     zerolog.Logger
	version    string
}

// New creates a Collector.
func New(s *store.Store, client *transparencia.Client, logger zerolog.Logger, version string) *Collector {
	return &Collector{
		store:      s,
		httpClient: client,
		logger:     logger,
		version:    version,
	}
}

// RunOnce executes a single collection cycle for the given source.
func (c *Collector) RunOnce(ctx context.Context, sourceID string, fetcher Fetcher) (err error) {
	log := c.logger.With().Str("source", sourceID).Logger()
	log.Info().Msg("collector: starting run")
	start := time.Now()

	defer func() {
		metrics.CollectorRunDuration.WithLabelValues(sourceID).Observe(time.Since(start).Seconds())
		status := "ok"
		if err != nil {
			status = "error"
		}
		metrics.CollectorRunsTotal.WithLabelValues(sourceID, status).Inc()
	}()

	if _, err := c.store.GetSource(ctx, sourceID); err != nil {
		return fmt.Errorf("collector: source %q not registered: %w", sourceID, err)
	}

	result, err := fetcher(ctx, c.httpClient)
	if err != nil {
		return fmt.Errorf("collector: fetch %q: %w", sourceID, err)
	}
	log.Info().
		Int("records", len(result.Records)).
		Int("pages", result.TotalPages).
		Int64("bytes", result.TotalBytes).
		Msg("collector: fetch complete")

	if len(result.Records) == 0 {
		log.Warn().Msg("collector: no records returned, aborting")
		return errors.New("collector: zero records returned, snapshot not created")
	}

	// Canonicalize and hash each record.
	collectedAt := time.Now().UTC()
	events := make([]store.InsertEventParams, 0, len(result.Records))
	seen := make(map[string]struct{}, len(result.Records))
	var totalBytes int64

	for i, rec := range result.Records {
		if _, dup := seen[rec.ExternalID]; dup {
			log.Warn().Str("external_id", rec.ExternalID).Msg("collector: duplicate external id in page, skipping")
			continue
		}
		seen[rec.ExternalID] = struct{}{}

		canonical, err := canonjson.Marshal(decodeAny(rec.Raw))
		if err != nil {
			return fmt.Errorf("collector: canonicalize record %d (%s): %w", i, rec.ExternalID, err)
		}
		hash := sha256.Sum256(canonical)
		events = append(events, store.InsertEventParams{
			SourceID:      sourceID,
			ExternalID:    rec.ExternalID,
			ContentHash:   hash[:],
			CanonicalJSON: canonical,
		})
		totalBytes += int64(len(canonical))
	}

	// Create the snapshot row with final counts.
	snapshotID, err := c.store.CreateSnapshotWithCounts(ctx, store.CreateSnapshotParams{
		SourceID:         sourceID,
		CollectedAt:      collectedAt,
		APIVersion:       result.APIVersion,
		CollectorVersion: c.version,
	}, len(events), totalBytes, "")
	if err != nil {
		return fmt.Errorf("collector: create snapshot: %w", err)
	}

	// Attach snapshot_id and insert all events.
	for i := range events {
		events[i].SnapshotID = snapshotID
	}
	inserted, err := c.store.InsertEventsBatch(ctx, events)
	if err != nil {
		return fmt.Errorf("collector: insert events (%d of %d inserted): %w", inserted, len(events), err)
	}

	metrics.CollectorRecordsTotal.WithLabelValues(sourceID).Add(float64(inserted))

	log.Info().
		Int64("snapshot_id", snapshotID).
		Int("inserted", inserted).
		Dur("elapsed", time.Since(start)).
		Msg("collector: run complete")
	return nil
}

// decodeAny turns raw JSON bytes into a Go value suitable for canonjson.Marshal.
// canonjson handles re-encoding deterministically, so this is just an unmarshal.
func decodeAny(raw []byte) any {
	return rawMessage(raw)
}

// rawMessage implements json.Marshaler so canonjson can round-trip the bytes.
type rawMessage []byte

// MarshalJSON makes rawMessage behave like json.RawMessage.
func (r rawMessage) MarshalJSON() ([]byte, error) {
	if len(r) == 0 {
		return []byte("null"), nil
	}
	return []byte(r), nil
}
