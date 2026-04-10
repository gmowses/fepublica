// Command notifier dispatches change_events to external channels:
// RSS (marks-only), Telegram, and Mastodon. Each channel is independently
// configurable via environment variables; missing credentials disable the
// corresponding channel silently.
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
	"github.com/gmowses/fepublica/internal/notify"
	"github.com/gmowses/fepublica/internal/store"
)

var version = "dev"

func main() {
	var once bool
	var interval time.Duration
	flag.BoolVar(&once, "once", false, "run a single pass and exit")
	flag.DurationVar(&interval, "interval", 30*time.Second, "loop interval")
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
	log := logging.New(cfg.Log.Level, cfg.Log.Format, "notifier")
	log.Info().
		Str("version", version).
		Dur("interval", interval).
		Bool("once", once).
		Bool("telegram_enabled", cfg.Telegram.BotToken != "" && cfg.Telegram.ChannelID != "").
		Bool("mastodon_enabled", cfg.Mastodon.InstanceURL != "" && cfg.Mastodon.AccessToken != "").
		Msg("notifier: starting")

	st, err := store.Open(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer st.Close()

	tg := notify.NewTelegramChannel(cfg.Telegram.BotToken, cfg.Telegram.ChannelID)
	ma := notify.NewMastodonChannel(cfg.Mastodon.InstanceURL, cfg.Mastodon.AccessToken)
	channels := []notify.Channel{
		notify.NewRSSChannel(),
		tg,
		ma,
	}

	d := notify.New(st, log, channels...)
	fd := notify.NewFindingsDispatcher(st, tg, ma, cfg.API.BaseURL)

	if once {
		if _, err := fd.RunOnce(ctx); err != nil {
			log.Error().Err(err).Msg("notifier: findings pass failed")
		}
		return d.RunOnce(ctx, 0)
	}
	go fd.Loop(ctx, interval)
	d.Loop(ctx, interval)
	return nil
}
