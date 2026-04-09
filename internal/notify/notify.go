// Package notify dispatches change_events to external channels:
// Telegram, Mastodon, and (internally) the RSS/JSON feeds.
//
// The RSS/JSON "channels" are served by the API by reading change_events
// directly — the notifier marks events as published_rss=true once they've
// been included in a rendered feed window. This is enough for the current
// pull model; a future push-based RSS could emit WebSub notifications.
package notify

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"

	"github.com/gmowses/fepublica/internal/store"
)

// Channel is implemented by every delivery target.
type Channel interface {
	Name() string
	// Publish delivers a single event. Returns nil on success, or an error
	// on transient/permanent failure. The dispatcher continues with the
	// next channel regardless.
	Publish(ctx context.Context, event *store.ChangeEvent) error
	// MarkField is the column name in change_events that tracks delivery
	// on this channel (e.g. "published_rss", "published_telegram").
	MarkField() string
}

// ErrChannelSkipped is returned by channels that are configured as disabled.
// The dispatcher treats this as "not an error, just don't mark it delivered".
var ErrChannelSkipped = errors.New("notify: channel disabled")

// Dispatcher iterates over configured channels and publishes pending events.
type Dispatcher struct {
	store    *store.Store
	channels []Channel
	logger   zerolog.Logger
}

// New builds a Dispatcher with the given channels.
func New(s *store.Store, logger zerolog.Logger, channels ...Channel) *Dispatcher {
	return &Dispatcher{
		store:    s,
		channels: channels,
		logger:   logger,
	}
}

// RunOnce fetches a batch of unpublished events per channel and publishes them.
// Events are selected independently per channel so a slow channel doesn't
// block the others.
func (d *Dispatcher) RunOnce(ctx context.Context, batch int) error {
	if batch <= 0 {
		batch = 50
	}
	for _, ch := range d.channels {
		events, err := d.store.ListUnpublishedChangeEvents(ctx, ch.MarkField(), batch)
		if err != nil {
			d.logger.Error().Err(err).Str("channel", ch.Name()).Msg("notify: list unpublished failed")
			continue
		}
		if len(events) == 0 {
			continue
		}
		d.logger.Info().
			Str("channel", ch.Name()).
			Int("pending", len(events)).
			Msg("notify: publishing batch")

		for i := range events {
			ev := &events[i]
			if err := ch.Publish(ctx, ev); err != nil {
				if errors.Is(err, ErrChannelSkipped) {
					// Disabled channel — mark as published so we don't retry forever.
					_ = d.store.MarkChangeEventPublished(ctx, ev.ID, ch.MarkField())
					continue
				}
				d.logger.Error().
					Err(err).
					Str("channel", ch.Name()).
					Int64("event_id", ev.ID).
					Msg("notify: publish failed")
				continue
			}
			if err := d.store.MarkChangeEventPublished(ctx, ev.ID, ch.MarkField()); err != nil {
				d.logger.Error().Err(err).Int64("event_id", ev.ID).Msg("notify: mark published failed")
			}
		}
	}
	return nil
}

// Loop runs RunOnce every interval until the context is cancelled.
func (d *Dispatcher) Loop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	_ = d.RunOnce(ctx, 0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = d.RunOnce(ctx, 0)
		}
	}
}
