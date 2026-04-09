// Command driftd runs the drift detector worker.
//
// On each pass it looks for pairs of consecutive snapshots of the same source
// that have not yet been compared, computes the diff, and persists a
// diff_run plus one change_event per affected record. Runs in a loop until
// interrupted, with an optional --once flag for testing.
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
	"github.com/gmowses/fepublica/internal/drift"
	"github.com/gmowses/fepublica/internal/logging"
	"github.com/gmowses/fepublica/internal/store"
)

var version = "dev"

func main() {
	var once bool
	var interval time.Duration
	flag.BoolVar(&once, "once", false, "run a single pass and exit")
	flag.DurationVar(&interval, "interval", 2*time.Minute, "loop interval between batches")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, once, interval); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, once bool, interval time.Duration) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.Log.Level, cfg.Log.Format, "driftd")
	log.Info().
		Str("version", version).
		Dur("interval", interval).
		Bool("once", once).
		Msg("driftd: starting")

	st, err := store.Open(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer st.Close()

	d := drift.New(st, log)
	if once {
		processed, err := d.RunOnce(ctx, 0)
		if err != nil {
			return err
		}
		log.Info().Int("processed", processed).Msg("driftd: done (once)")
		return nil
	}
	d.Loop(ctx, interval)
	return nil
}
