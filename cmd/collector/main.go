// Command collector runs scheduled data collection from public Brazilian sources.
//
// Usage:
//
//	collector run --source ceis     # run one collection cycle for CEIS
//	collector run --source cnep     # run one collection cycle for CNEP
//	collector serve                 # run scheduled loop driven by cron expressions
//
// Configuration is loaded from environment variables. See internal/config for defaults.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/gmowses/fepublica/internal/collector"
	"github.com/gmowses/fepublica/internal/config"
	"github.com/gmowses/fepublica/internal/logging"
	"github.com/gmowses/fepublica/internal/store"
	"github.com/gmowses/fepublica/internal/transparencia"
	"github.com/gmowses/fepublica/internal/transparencia/ceis"
	"github.com/gmowses/fepublica/internal/transparencia/cnep"
	"github.com/gmowses/fepublica/internal/transparencia/pncp"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:   "collector",
		Short: "Fé Pública — public data collector",
	}

	var sourceFlag string

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run one collection cycle for a specific source",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sourceFlag == "" {
				return fmt.Errorf("--source is required (ceis, cnep, pncp-contratos)")
			}
			return runOnce(cmd.Context(), sourceFlag)
		},
	}
	runCmd.Flags().StringVar(&sourceFlag, "source", "", "source id (ceis, cnep, pncp-contratos)")

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Run scheduled collection loop",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serve(cmd.Context())
		},
	}

	root.AddCommand(runCmd, serveCmd)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runOnce(ctx context.Context, sourceID string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.Log.Level, cfg.Log.Format, "collector")

	st, err := store.Open(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer st.Close()

	client := transparencia.New(cfg.Transparencia.APIKey,
		transparencia.WithUserAgent(cfg.Transparencia.UserAgent))
	c := collector.New(st, client, log, version)

	fetcher, err := resolveFetcher(sourceID)
	if err != nil {
		return err
	}
	return c.RunOnce(ctx, sourceID, fetcher)
}

func serve(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.Log.Level, cfg.Log.Format, "collector")

	st, err := store.Open(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer st.Close()

	client := transparencia.New(cfg.Transparencia.APIKey,
		transparencia.WithUserAgent(cfg.Transparencia.UserAgent))
	c := collector.New(st, client, log, version)

	sched := cron.New(cron.WithLogger(cronLogger{log}))
	if _, err := sched.AddFunc(cfg.Collector.CEISSchedule, func() {
		if err := c.RunOnce(ctx, ceis.SourceID, ceis.Fetch); err != nil {
			log.Error().Err(err).Str("source", ceis.SourceID).Msg("scheduled run failed")
		}
	}); err != nil {
		return fmt.Errorf("schedule ceis: %w", err)
	}
	if _, err := sched.AddFunc(cfg.Collector.CNEPSchedule, func() {
		if err := c.RunOnce(ctx, cnep.SourceID, cnep.Fetch); err != nil {
			log.Error().Err(err).Str("source", cnep.SourceID).Msg("scheduled run failed")
		}
	}); err != nil {
		return fmt.Errorf("schedule cnep: %w", err)
	}
	if _, err := sched.AddFunc(cfg.Collector.PNCPSchedule, func() {
		if err := c.RunOnce(ctx, pncp.SourceID, pncp.Fetch); err != nil {
			log.Error().Err(err).Str("source", pncp.SourceID).Msg("scheduled run failed")
		}
	}); err != nil {
		return fmt.Errorf("schedule pncp: %w", err)
	}

	log.Info().
		Str("ceis_schedule", cfg.Collector.CEISSchedule).
		Str("cnep_schedule", cfg.Collector.CNEPSchedule).
		Str("pncp_schedule", cfg.Collector.PNCPSchedule).
		Msg("collector: scheduler started")

	sched.Start()
	<-ctx.Done()
	stopCtx := sched.Stop()
	<-stopCtx.Done()
	return nil
}

func resolveFetcher(sourceID string) (collector.Fetcher, error) {
	switch sourceID {
	case ceis.SourceID:
		return ceis.Fetch, nil
	case cnep.SourceID:
		return cnep.Fetch, nil
	case pncp.SourceID:
		return pncp.Fetch, nil
	default:
		return nil, fmt.Errorf("unknown source %q (supported: ceis, cnep, pncp-contratos)", sourceID)
	}
}

// cronLogger adapts our zerolog to robfig/cron's logger interface.
type cronLogger struct {
	zl any // zerolog.Logger kept as any to avoid the package depending on cron's logger surface
}

func (c cronLogger) Info(msg string, keysAndValues ...any)             {}
func (c cronLogger) Error(err error, msg string, keysAndValues ...any) {}
