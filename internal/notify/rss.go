package notify

import (
	"context"

	"github.com/gmowses/fepublica/internal/store"
)

// RSSChannel is a no-op publisher: the API serves RSS/JSON feeds by reading
// change_events on demand. This channel only marks the published_rss flag so
// we know which events were included in a "feed window" for accounting.
type RSSChannel struct{}

// NewRSSChannel returns a ready-to-use RSS/JSON publisher.
func NewRSSChannel() RSSChannel { return RSSChannel{} }

// Name implements Channel.
func (RSSChannel) Name() string { return "rss" }

// MarkField implements Channel.
func (RSSChannel) MarkField() string { return "published_rss" }

// Publish implements Channel. The RSS/JSON feed is a pull endpoint, so there
// is nothing to push — but we still mark events as "included" so the API can
// optimize queries like "give me new events that haven't been feed-ready yet".
func (RSSChannel) Publish(ctx context.Context, event *store.ChangeEvent) error {
	return nil
}
