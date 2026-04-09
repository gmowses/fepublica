// Command anchor runs the anchor worker loop.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gmowses/fepublica/internal/anchor"
	"github.com/gmowses/fepublica/internal/config"
	"github.com/gmowses/fepublica/internal/logging"
	"github.com/gmowses/fepublica/internal/store"
)

var version = "dev"

func main() {
	var once bool
	flag.BoolVar(&once, "once", false, "run a single pass and exit (useful for testing)")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, once); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, once bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.Log.Level, cfg.Log.Format, "anchor")
	log.Info().
		Str("version", version).
		Dur("interval", cfg.Anchor.BatchInterval).
		Bool("once", once).
		Msg("anchor: starting")

	st, err := store.Open(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer st.Close()

	w := anchor.New(st, cfg.OTS.Calendars, log)
	if once {
		return w.RunOnce(ctx)
	}
	w.Loop(ctx, cfg.Anchor.BatchInterval)
	return nil
}
