// Command archive runs the cold archive worker: it moves old event payloads
// from the Postgres `events.canonical_json` column into the S3-compatible
// bucket, reclaiming database space while preserving all cryptographic
// proofs (content_hash stays in place).
//
// The worker is safe to run repeatedly and on any schedule. By default it
// looks for events older than EVENTS_HOT_RETENTION_DAYS (default 90) and
// processes up to ARCHIVE_BATCH_SIZE (default 1000) events per pass.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gmowses/fepublica/internal/archive"
	"github.com/gmowses/fepublica/internal/config"
	"github.com/gmowses/fepublica/internal/logging"
	"github.com/gmowses/fepublica/internal/store"
)

var version = "dev"

func main() {
	var once bool
	var interval time.Duration
	var retention int
	flag.BoolVar(&once, "once", false, "run a single pass and exit")
	flag.DurationVar(&interval, "interval", 24*time.Hour, "loop interval between archive passes")
	flag.IntVar(&retention, "retention-days", 90, "keep events in postgres for this many days before archiving")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, once, interval, retention); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, once bool, interval time.Duration, retention int) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.Log.Level, cfg.Log.Format, "archive")
	log.Info().
		Str("version", version).
		Bool("once", once).
		Dur("interval", interval).
		Int("retention_days", retention).
		Str("bucket", cfg.S3.Bucket).
		Msg("archive: starting")

	if cfg.S3.Bucket == "" {
		log.Warn().Msg("archive: S3_BUCKET not configured — worker will exit")
		return nil
	}

	st, err := store.Open(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer st.Close()

	ac, err := archive.New(archive.Config{
		Endpoint:  cfg.S3.Endpoint,
		Region:    cfg.S3.Region,
		AccessKey: cfg.S3.AccessKey,
		SecretKey: cfg.S3.SecretKey,
		Bucket:    cfg.S3.Bucket,
		UseSSL:    true,
	})
	if err != nil {
		return err
	}

	runOnce := func() error {
		cutoff := time.Now().AddDate(0, 0, -retention)
		events, err := st.ListColdEvents(ctx, cutoff, 1000)
		if err != nil {
			return fmt.Errorf("list cold: %w", err)
		}
		if len(events) == 0 {
			log.Info().Msg("archive: nothing to archive")
			return nil
		}
		log.Info().Int("count", len(events)).Time("cutoff", cutoff).Msg("archive: uploading batch")

		uploaded := 0
		for _, ev := range events {
			key := fmt.Sprintf("events/%s/%d/%s.json", ev.SourceID, ev.SnapshotID, ev.ExternalID)
			err := ac.Put(ctx, key, bytes.NewReader(ev.CanonicalJSON), int64(len(ev.CanonicalJSON)), "application/json")
			if err != nil {
				log.Error().Err(err).Int64("event_id", ev.ID).Msg("archive: upload failed")
				continue
			}
			if err := st.MarkEventArchived(ctx, ev.ID, "s3://"+ac.Bucket()+"/"+key); err != nil {
				log.Error().Err(err).Int64("event_id", ev.ID).Msg("archive: mark archived failed")
				continue
			}
			uploaded++
		}
		log.Info().Int("uploaded", uploaded).Msg("archive: batch complete")
		return nil
	}

	if once {
		return runOnce()
	}

	t := time.NewTicker(interval)
	defer t.Stop()
	_ = runOnce()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := runOnce(); err != nil {
				log.Error().Err(err).Msg("archive: run failed")
			}
		}
	}
}
