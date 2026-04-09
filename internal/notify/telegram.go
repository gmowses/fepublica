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

// TelegramChannel posts change_events to a public Telegram channel via the Bot API.
// If botToken or channelID is empty, the channel is disabled and Publish
// returns ErrChannelSkipped for every event.
type TelegramChannel struct {
	botToken  string
	channelID string
	baseURL   string // defaults to "https://api.telegram.org"
	client    *http.Client
}

// NewTelegramChannel constructs the channel. Pass empty strings to disable.
func NewTelegramChannel(botToken, channelID string) *TelegramChannel {
	return &TelegramChannel{
		botToken:  botToken,
		channelID: channelID,
		baseURL:   "https://api.telegram.org",
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Name implements Channel.
func (*TelegramChannel) Name() string { return "telegram" }

// MarkField implements Channel.
func (*TelegramChannel) MarkField() string { return "published_telegram" }

// Publish sends a formatted message to the Telegram channel.
func (t *TelegramChannel) Publish(ctx context.Context, event *store.ChangeEvent) error {
	if t.botToken == "" || t.channelID == "" {
		return ErrChannelSkipped
	}

	text := t.formatText(event)
	params := url.Values{}
	params.Set("chat_id", t.channelID)
	params.Set("text", text)
	params.Set("parse_mode", "MarkdownV2")
	params.Set("disable_web_page_preview", "false")

	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, t.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("telegram: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: do request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		// Extract description for nicer logging if possible.
		var parsed struct {
			OK          bool   `json:"ok"`
			Description string `json:"description"`
		}
		_ = json.Unmarshal(body, &parsed)
		return fmt.Errorf("telegram: status %d: %s", resp.StatusCode, parsed.Description)
	}
	return nil
}

func (*TelegramChannel) formatText(e *store.ChangeEvent) string {
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

	// MarkdownV2 escapes — periods, hyphens, parentheses, etc.
	escape := func(s string) string {
		repl := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
		out := s
		for _, c := range repl {
			out = strings.ReplaceAll(out, c, "\\"+c)
		}
		return out
	}

	return fmt.Sprintf("%s *%s*%s\n`%s`\nexternal\\_id: `%s`\nFé Pública · %s",
		emoji,
		escape(strings.ToUpper(e.SourceID)),
		sev,
		escape(e.ChangeType),
		escape(e.ExternalID),
		escape(e.DetectedAt.UTC().Format(time.RFC3339)),
	)
}
