// Command lai-crawler checks LAI compliance for configured public entes.
//
// MVP scope: runs the tier-1 cohort (federal + UFs + curated bodies, ~55
// entes) on every pass, 1 request per second, respects robots.txt via the
// HTTP client's user-agent. Writes a row to lai_checks and updates lai_scores.
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
	"github.com/gmowses/fepublica/internal/lai"
	"github.com/gmowses/fepublica/internal/logging"
	"github.com/gmowses/fepublica/internal/store"
)

var version = "dev"

func main() {
	var once bool
	var interval time.Duration
	var tier int
	var batch int
	flag.BoolVar(&once, "once", false, "run a single pass and exit")
	flag.DurationVar(&interval, "interval", 6*time.Hour, "loop interval between passes")
	flag.IntVar(&tier, "tier", 1, "ente tier to check (0 = all)")
	flag.IntVar(&batch, "batch", 100, "max entes per pass")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, once, interval, tier, batch); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, once bool, interval time.Duration, tier, batch int) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.Log.Level, cfg.Log.Format, "lai-crawler")
	log.Info().
		Str("version", version).
		Bool("once", once).
		Int("tier", tier).
		Int("batch", batch).
		Msg("lai-crawler: starting")

	st, err := store.Open(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer st.Close()

	checker := lai.NewChecker()

	runOnce := func() error {
		entes, err := st.ListEntesForCrawl(ctx, tier, batch)
		if err != nil {
			return err
		}
		log.Info().Int("count", len(entes)).Msg("lai-crawler: batch")
		rateLimiter := time.NewTicker(1 * time.Second)
		defer rateLimiter.Stop()
		for i := range entes {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-rateLimiter.C:
			}
			ente := &entes[i]
			r := checker.Check(ctx, ente)
			checkID, err := st.InsertLaiCheck(ctx, toStoreCheck(r))
			if err != nil {
				log.Error().Err(err).Str("ente", ente.ID).Msg("lai-crawler: insert failed")
				continue
			}
			score, components := lai.Score(r)
			if err := st.UpsertLaiScore(ctx, ente.ID, score, components, checkID); err != nil {
				log.Error().Err(err).Str("ente", ente.ID).Msg("lai-crawler: score upsert failed")
			}
			log.Info().
				Str("ente", ente.ID).
				Int("status", r.HTTPStatus).
				Int("response_ms", r.ResponseMS).
				Float64("score", score).
				Msg("lai-crawler: checked")
		}
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
				log.Error().Err(err).Msg("lai-crawler: pass failed")
			}
		}
	}
}

func toStoreCheck(r *lai.Result) *store.LaiCheck {
	return &store.LaiCheck{
		EnteID:         r.EnteID,
		CheckedAt:      r.CheckedAt,
		TargetURL:      r.TargetURL,
		HTTPStatus:     r.HTTPStatus,
		ResponseMS:     r.ResponseMS,
		SSLValid:       r.SSLValid,
		SSLExpiresAt:   r.SSLExpiresAt,
		PortalLoads:    r.PortalLoads,
		HTMLSizeBytes:  r.HTMLSizeBytes,
		TermsFound:     r.TermsFound,
		RequiredLinks:  r.RequiredLinks,
		HTMLArchiveKey: r.HTMLArchiveKey,
		Errors:         r.Errors,
		TierAtCheck:    r.TierAtCheck,
	}
}
