// Command enteadm is a one-shot CLI that seeds the entes table from IBGE
// and the curated federal YAML.
//
// Usage:
//
//	enteadm seed [--federal-yaml db/seeds/entes-federal.yaml]
//
// Safe to re-run: everything is upsert-by-id, and IBGE refresh just bumps
// updated_at on municipalities.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gmowses/fepublica/internal/config"
	"github.com/gmowses/fepublica/internal/entes"
	"github.com/gmowses/fepublica/internal/logging"
	"github.com/gmowses/fepublica/internal/store"
)

var version = "dev"

func main() {
	var federalYAML string
	flag.StringVar(&federalYAML, "federal-yaml", "db/seeds/entes-federal.yaml", "path to federal YAML seed file")
	flag.Parse()

	cmd := ""
	if flag.NArg() > 0 {
		cmd = flag.Arg(0)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if cmd != "seed" {
		fmt.Fprintln(os.Stderr, "usage: enteadm seed [--federal-yaml path]")
		os.Exit(2)
	}

	if err := run(ctx, federalYAML); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, federalYAML string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.Log.Level, cfg.Log.Format, "enteadm")
	log.Info().Str("version", version).Msg("enteadm: starting seed")

	st, err := store.Open(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer st.Close()

	seeder := entes.NewSeeder(st, log)
	if err := seeder.Run(ctx, federalYAML); err != nil {
		return err
	}
	total, err := st.CountEntes(ctx)
	if err != nil {
		return err
	}
	log.Info().Int("total_entes", total).Msg("enteadm: seed complete")
	return nil
}
