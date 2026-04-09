package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gmowses/fepublica/internal/store"
)

// MastodonChannel posts change_events as statuses to a Mastodon instance.
// If instanceURL or accessToken is empty, the channel is disabled and Publish
// returns ErrChannelSkipped.
type MastodonChannel struct {
	instanceURL string // e.g. "https://mastodon.social"
	accessToken string
	client      *http.Client
}

// NewMastodonChannel constructs the channel.
func NewMastodonChannel(instanceURL, accessToken string) *MastodonChannel {
	return &MastodonChannel{
		instanceURL: strings.TrimRight(instanceURL, "/"),
		accessToken: accessToken,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

// Name implements Channel.
func (*MastodonChannel) Name() string { return "mastodon" }

// MarkField implements Channel.
func (*MastodonChannel) MarkField() string { return "published_mastodon" }

// Publish sends a status to the Mastodon instance.
func (m *MastodonChannel) Publish(ctx context.Context, event *store.ChangeEvent) error {
	if m.instanceURL == "" || m.accessToken == "" {
		return ErrChannelSkipped
	}

	status := m.formatStatus(event)
	params := url.Values{}
	params.Set("status", status)
	params.Set("visibility", "public")

	apiURL := m.instanceURL + "/api/v1/statuses"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("mastodon: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+m.accessToken)
	req.Header.Set("Idempotency-Key", fmt.Sprintf("fepublica-ce-%d", event.ID))

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("mastodon: do request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mastodon: status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return nil
}

func (*MastodonChannel) formatStatus(e *store.ChangeEvent) string {
	emoji := map[string]string{
		"added":    "🆕",
		"removed":  "🗑️",
		"modified": "✏️",
	}[e.ChangeType]

	sev := ""
	if e.Severity == "warn" {
		sev = " ⚠️"
	} else if e.Severity == "alert" {
		sev = " 🚨"
	}

	link := fmt.Sprintf("https://fepublica.gmowses.cloud/recent?source=%s", e.SourceID)
	return fmt.Sprintf(
		"%s %s%s\n\n%s: %s\nfonte: %s\ndetectado: %s\n\n%s\n\n#fepublica #accountability #dadosabertos",
		emoji,
		strings.ToUpper(e.SourceID),
		sev,
		e.ChangeType,
		e.ExternalID,
		e.SourceID,
		e.DetectedAt.UTC().Format(time.RFC3339),
		link,
	)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
