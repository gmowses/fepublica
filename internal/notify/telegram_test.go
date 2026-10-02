package notify

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gmowses/fepublica/internal/store"
)

func TestTelegramRequestErrorDoesNotExposeURL(t *testing.T) {
	err := telegramRequestError(errors.New("https://api.telegram.org/botsecret/sendMessage: failed"))
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "telegram.org") {
		t.Fatalf("telegram error exposed request URL: %q", err)
	}
}

func TestTelegramRequestBuildErrorDoesNotExposeToken(t *testing.T) {
	channel := NewTelegramChannel("secret-token", "channel")
	channel.baseURL = "\x7f"
	err := channel.Publish(context.Background(), &store.ChangeEvent{})
	if err == nil {
		t.Fatal("Publish succeeded with an invalid Telegram URL")
	}
	if strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), "\x7f") {
		t.Fatalf("telegram error exposed request details: %q", err)
	}
}
