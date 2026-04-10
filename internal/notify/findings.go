package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gmowses/fepublica/internal/store"
)

// FindingsDispatcher pushes new high-severity findings to telegram/mastodon
// using the same credentials as the regular notifier. Has its own pass loop
// because it tracks delivery via findings.notified_at, not change_events.
type FindingsDispatcher struct {
	store    *store.Store
	telegram *TelegramChannel
	mastodon *MastodonChannel
	baseURL  string // public app URL for building deep links
}

// NewFindingsDispatcher constructs the dispatcher.
func NewFindingsDispatcher(s *store.Store, tg *TelegramChannel, ma *MastodonChannel, baseURL string) *FindingsDispatcher {
	return &FindingsDispatcher{store: s, telegram: tg, mastodon: ma, baseURL: baseURL}
}

// RunOnce pulls pending high findings and notifies. Idempotent: only marks
// finding.notified_at on success.
func (d *FindingsDispatcher) RunOnce(ctx context.Context) (int, error) {
	pending, err := d.store.ListPendingNotificationFindings(ctx, 20)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, pf := range pending {
		title, body, link := formatFinding(pf, d.baseURL)
		ok := false

		if d.telegram != nil && d.telegram.botToken != "" && d.telegram.channelID != "" {
			if err := postTelegramText(ctx, d.telegram, title, body, link); err == nil {
				ok = true
			}
		}
		if d.mastodon != nil && d.mastodon.instanceURL != "" && d.mastodon.accessToken != "" {
			if err := postMastodonText(ctx, d.mastodon, title+"\n\n"+body+"\n"+link); err == nil {
				ok = true
			}
		}

		if ok || (d.telegram == nil && d.mastodon == nil) {
			if err := d.store.MarkFindingNotified(ctx, pf.ID); err != nil {
				return sent, err
			}
			sent++
		}
	}
	return sent, nil
}

// Loop runs RunOnce in a loop.
func (d *FindingsDispatcher) Loop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	_, _ = d.RunOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, _ = d.RunOnce(ctx)
		}
	}
}

func formatFinding(pf store.PersistedFinding, baseURL string) (title, body, link string) {
	emoji := "🚨"
	title = emoji + " " + pf.Finding.Title

	body = pf.Finding.Subject
	if pf.Finding.Valor != nil {
		body += fmt.Sprintf(" — R$ %s", formatMoney(*pf.Finding.Valor))
	}
	if expl, ok := pf.Finding.Evidence["explanation"].(string); ok {
		body += "\n\n" + expl
	}
	if pf.Finding.Link != "" {
		link = baseURL + pf.Finding.Link
	} else {
		link = baseURL + "/forenses"
	}
	return
}

func formatMoney(v float64) string {
	if v >= 1e6 {
		return fmt.Sprintf("%.1fM", v/1e6)
	}
	if v >= 1e3 {
		return fmt.Sprintf("%.0fk", v/1e3)
	}
	return fmt.Sprintf("%.0f", v)
}

// postTelegramText sends an arbitrary text message to the configured channel.
// Reuses TelegramChannel's HTTP/credentials but bypasses the ChangeEvent
// formatter.
func postTelegramText(ctx context.Context, t *TelegramChannel, title, body, link string) error {
	escape := func(s string) string {
		for _, c := range []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"} {
			s = strings.ReplaceAll(s, c, "\\"+c)
		}
		return s
	}
	text := fmt.Sprintf("*%s*\n%s\n\n[%s](%s)\nFé Pública · forenses",
		escape(title), escape(body), escape("investigar"), link)

	params := url.Values{}
	params.Set("chat_id", t.channelID)
	params.Set("text", text)
	params.Set("parse_mode", "MarkdownV2")
	params.Set("disable_web_page_preview", "false")

	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, t.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body2, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode != http.StatusOK {
		var parsed struct {
			Description string `json:"description"`
		}
		_ = json.Unmarshal(body2, &parsed)
		return fmt.Errorf("telegram: status %d: %s", resp.StatusCode, parsed.Description)
	}
	return nil
}

// postMastodonText posts a status to Mastodon using the configured token.
func postMastodonText(ctx context.Context, m *MastodonChannel, text string) error {
	if len(text) > 480 {
		text = text[:480] + "..."
	}
	params := url.Values{}
	params.Set("status", text)
	params.Set("visibility", "public")

	apiURL := strings.TrimRight(m.instanceURL, "/") + "/api/v1/statuses"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.accessToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("mastodon: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
