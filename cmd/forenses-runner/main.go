// Command forenses-runner roda todos os detectores periodicamente e
// persiste os findings na tabela findings, idempotente via dedup_key.
//
// Modes:
//   --once       run a single pass and exit
//   --interval   loop interval (default 30m)
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
	"github.com/gmowses/fepublica/internal/logging"
	"github.com/gmowses/fepublica/internal/store"
)

var version = "dev"

func main() {
	var once bool
	var interval time.Duration
	flag.BoolVar(&once, "once", false, "run a single pass and exit")
	flag.DurationVar(&interval, "interval", 30*time.Minute, "loop interval between passes")
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
	log := logging.New(cfg.Log.Level, cfg.Log.Format, "forenses-runner")
	log.Info().Str("version", version).Bool("once", once).Dur("interval", interval).Msg("starting")

	st, err := store.Open(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer st.Close()

	runOnce := func() error {
		// Run each detector and accumulate findings.
		var all []store.Finding

		for name, fn := range map[string]func(context.Context, int) ([]store.Finding, error){
			"sancionados":  st.FindSancionadosContratados,
			"concentracao": st.FindConcentracaoOrgao,
			"outliers":     st.FindValorOutliers,
			"cpgf_alto":    st.FindCPGFAltoValor,
			"cpgf_opaco":   st.FindCPGFEstabOpaco,
		} {
			start := time.Now()
			rows, err := fn(ctx, 200)
			if err != nil {
				log.Error().Err(err).Str("detector", name).Msg("detector failed")
				continue
			}
			log.Info().
				Str("detector", name).
				Int("count", len(rows)).
				Dur("dur", time.Since(start)).
				Msg("detector ran")
			all = append(all, rows...)
		}

		newIDs, err := st.PersistFindings(ctx, all)
		if err != nil {
			return fmt.Errorf("persist: %w", err)
		}
		log.Info().
			Int("total", len(all)).
			Int("new", len(newIDs)).
			Msg("pass complete")
		return nil
	}

	if once {
		return runOnce()
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	if err := runOnce(); err != nil {
		log.Error().Err(err).Msg("first pass failed")
	}
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
