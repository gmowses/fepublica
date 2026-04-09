// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for Fé Pública components.
type Config struct {
	Postgres      PostgresConfig
	Transparencia TransparenciaConfig
	OTS           OTSConfig
	Collector     CollectorConfig
	Anchor        AnchorConfig
	API           APIConfig
	Log           LogConfig
	Telegram      TelegramConfig
	Mastodon      MastodonConfig
	S3            S3Config
}

// TelegramConfig configures the Telegram bot channel. Empty values disable it.
type TelegramConfig struct {
	BotToken  string
	ChannelID string
}

// MastodonConfig configures the Mastodon publisher. Empty values disable it.
type MastodonConfig struct {
	InstanceURL string
	AccessToken string
}

// S3Config configures the S3-compatible object storage (IDrive E2 by default).
// Empty Bucket disables any archive worker that depends on it.
type S3Config struct {
	Endpoint  string
	Region    string
	AccessKey string
	SecretKey string
	Bucket    string
}

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// DSN returns a libpq-style connection string.
func (p PostgresConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.Database, p.SSLMode)
}

type TransparenciaConfig struct {
	APIKey    string
	UserAgent string
}

type OTSConfig struct {
	Calendars []string
}

type CollectorConfig struct {
	CEISSchedule string
	CNEPSchedule string
	PNCPSchedule string
}

type AnchorConfig struct {
	BatchInterval time.Duration
}

type APIConfig struct {
	Host    string
	Port    int
	BaseURL string
}

type LogConfig struct {
	Level  string
	Format string
}

// Load reads configuration from environment variables and validates required fields.
// Missing non-required fields are filled with defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Postgres: PostgresConfig{
			Host:     getenv("POSTGRES_HOST", "localhost"),
			Port:     getenvInt("POSTGRES_PORT", 5432),
			User:     getenv("POSTGRES_USER", "fepublica"),
			Password: getenv("POSTGRES_PASSWORD", ""),
			Database: getenv("POSTGRES_DB", "fepublica"),
			SSLMode:  getenv("POSTGRES_SSLMODE", "disable"),
		},
		Transparencia: TransparenciaConfig{
			APIKey:    getenv("TRANSPARENCIA_API_KEY", ""),
			UserAgent: getenv("TRANSPARENCIA_USER_AGENT", "fepublica/0.1 (+https://github.com/gmowses/fepublica)"),
		},
		OTS: OTSConfig{
			Calendars: splitAndTrim(getenv("OTS_CALENDARS",
				"https://alice.btc.calendar.opentimestamps.org,https://bob.btc.calendar.opentimestamps.org")),
		},
		Collector: CollectorConfig{
			CEISSchedule: getenv("COLLECTOR_CEIS_SCHEDULE", "0 4 * * *"),
			CNEPSchedule: getenv("COLLECTOR_CNEP_SCHEDULE", "15 4 * * *"),
			PNCPSchedule: getenv("COLLECTOR_PNCP_SCHEDULE", "30 4 * * *"),
		},
		Anchor: AnchorConfig{
			BatchInterval: getenvDuration("ANCHOR_BATCH_INTERVAL", 6*time.Hour),
		},
		API: APIConfig{
			Host:    getenv("API_HOST", "0.0.0.0"),
			Port:    getenvInt("API_PORT", 8080),
			BaseURL: getenv("API_BASE_URL", "http://localhost:8080"),
		},
		Log: LogConfig{
			Level:  getenv("LOG_LEVEL", "info"),
			Format: getenv("LOG_FORMAT", "json"),
		},
		Telegram: TelegramConfig{
			BotToken:  getenv("TELEGRAM_BOT_TOKEN", ""),
			ChannelID: getenv("TELEGRAM_CHANNEL_ID", ""),
		},
		Mastodon: MastodonConfig{
			InstanceURL: getenv("MASTODON_INSTANCE_URL", ""),
			AccessToken: getenv("MASTODON_ACCESS_TOKEN", ""),
		},
		S3: S3Config{
			Endpoint:  getenv("S3_ENDPOINT", ""),
			Region:    getenv("S3_REGION", ""),
			AccessKey: getenv("S3_ACCESS_KEY", ""),
			SecretKey: getenv("S3_SECRET_KEY", ""),
			Bucket:    getenv("S3_BUCKET", ""),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate returns an error describing the first missing required field.
func (c *Config) Validate() error {
	var missing []string
	if c.Postgres.Password == "" {
		missing = append(missing, "POSTGRES_PASSWORD")
	}
	if c.Transparencia.APIKey == "" {
		missing = append(missing, "TRANSPARENCIA_API_KEY")
	}
	if len(c.OTS.Calendars) == 0 {
		missing = append(missing, "OTS_CALENDARS")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	return nil
}

// LoadPartial loads config without enforcing full validation. Useful for components
// that only need a subset (e.g. the api binary doesn't need TRANSPARENCIA_API_KEY).
func LoadPartial() (*Config, error) {
	cfg, err := Load()
	if err == nil {
		return cfg, nil
	}
	// If only transparencia key is missing, tolerate it for api-only mode.
	var missingErr error = err
	if strings.Contains(err.Error(), "TRANSPARENCIA_API_KEY") &&
		!strings.Contains(err.Error(), "POSTGRES_PASSWORD") &&
		!strings.Contains(err.Error(), "OTS_CALENDARS") {
		return &Config{}, missingErr // caller can inspect err
	}
	return nil, err
}

func getenv(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getenvDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Errors for common config problems, exported for callers.
var (
	ErrMissingConfig = errors.New("missing required config")
)
