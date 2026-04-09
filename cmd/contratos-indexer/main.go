// Command contratos-indexer walks PNCP events and projects them into the
// contratos table. Idempotent: re-runs skip already-indexed rows via the
// LEFT JOIN filter in ListUnindexedPNCPEvents.
//
// Runs on a loop by default (every 5 minutes) so new PNCP collections get
// indexed continuously. --once for one-shot runs.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gmowses/fepublica/internal/config"
	"github.com/gmowses/fepublica/internal/contratos"
	"github.com/gmowses/fepublica/internal/logging"
	"github.com/gmowses/fepublica/internal/store"
)

var version = "dev"

func main() {
	var once bool
	var interval time.Duration
	var batch int
	flag.BoolVar(&once, "once", false, "run a single pass and exit")
	flag.DurationVar(&interval, "interval", 5*time.Minute, "loop interval between passes")
	flag.IntVar(&batch, "batch", 1000, "max events per pass")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, once, interval, batch); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, once bool, interval time.Duration, batch int) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.Log.Level, cfg.Log.Format, "contratos-indexer")
	log.Info().Str("version", version).Bool("once", once).Dur("interval", interval).Msg("starting")

	st, err := store.Open(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer st.Close()

	runOnce := func() error {
		events, err := st.ListUnindexedPNCPEvents(ctx, batch)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			log.Info().Msg("nothing to index")
			return nil
		}
		log.Info().Int("count", len(events)).Msg("indexing batch")
		indexed := 0
		failed := 0
		for i := range events {
			ev := &events[i]
			row, err := contratos.Parse(ev.CanonicalJSON, ev.ExternalID)
			if err != nil {
				log.Error().Err(err).Int64("event_id", ev.ID).Msg("parse failed")
				failed++
				continue
			}
			if err := st.InsertContrato(ctx, ev.ID, ev.SnapshotID, row, ev.CollectedAt); err != nil {
				log.Error().Err(err).Int64("event_id", ev.ID).Msg("insert failed")
				failed++
				continue
			}
			indexed++
		}
		log.Info().Int("indexed", indexed).Int("failed", failed).Msg("batch complete")
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
				log.Error().Err(err).Msg("pass failed")
			}
		}
	}
}
