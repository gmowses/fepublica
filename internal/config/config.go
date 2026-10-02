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
	CEISSchedule    string
	CNEPSchedule    string
	PNCPSchedule    string
	CartoesSchedule string
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
	cfg := load()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func load() *Config {
	return &Config{
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
			CEISSchedule:    getenv("COLLECTOR_CEIS_SCHEDULE", "0 4 * * *"),
			CNEPSchedule:    getenv("COLLECTOR_CNEP_SCHEDULE", "15 4 * * *"),
			PNCPSchedule:    getenv("COLLECTOR_PNCP_SCHEDULE", "30 4 * * *"),
			CartoesSchedule: getenv("COLLECTOR_CARTOES_SCHEDULE", "45 4 5 * *"),
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

}

// Validate returns an error describing the first missing required field.
func (c *Config) Validate() error {
	var missing []string
	if len(c.OTS.Calendars) == 0 {
		missing = append(missing, "OTS_CALENDARS")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	return c.ValidatePostgres()
}

// LoadPartial loads the database configuration required by API-only components.
func LoadPartial() (*Config, error) {
	cfg := load()
	if err := cfg.ValidatePostgres(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ValidatePostgres validates the database settings needed by API-only components.
func (c *Config) ValidatePostgres() error {
	if strings.TrimSpace(c.Postgres.Password) == "" {
		return errors.New("missing required config: POSTGRES_PASSWORD")
	}
	if isPlaceholder(c.Postgres.Password) {
		return errors.New("POSTGRES_PASSWORD must not use a placeholder value")
	}
	if !isLocalPostgresHost(c.Postgres.Host) {
		switch strings.ToLower(strings.TrimSpace(c.Postgres.SSLMode)) {
		case "require", "verify-ca", "verify-full":
		default:
			return errors.New("POSTGRES_SSLMODE must require TLS for a remote POSTGRES_HOST")
		}
	}
	return nil
}

func isPlaceholder(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	return v == "change_me_in_real_deployments" || v == "change_me" || v == "password" || v == "changeme" || strings.Contains(v, "replace_me")
}

func isLocalPostgresHost(host string) bool {
	if strings.HasPrefix(strings.TrimSpace(host), "/") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1", "postgres":
		return true
	default:
		return false
	}
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
