package config

import "testing"

func TestValidateRejectsPlaceholderPostgresPassword(t *testing.T) {
	cfg := &Config{Postgres: PostgresConfig{Host: "postgres", Password: "change_me_in_real_deployments", SSLMode: "disable"}, Transparencia: TransparenciaConfig{APIKey: "key"}, OTS: OTSConfig{Calendars: []string{"https://calendar.example"}}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted a placeholder Postgres password")
	}
}

func TestLoadPartialRejectsPlaceholderPostgresPassword(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "change_me")
	if _, err := LoadPartial(); err == nil {
		t.Fatal("LoadPartial accepted a placeholder Postgres password")
	}
}

func TestLoadPartialRequiresPostgresPassword(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "")
	if _, err := LoadPartial(); err == nil {
		t.Fatal("LoadPartial accepted an empty Postgres password")
	}
}

func TestValidateRequiresTLSForRemotePostgres(t *testing.T) {
	cfg := &Config{Postgres: PostgresConfig{Host: "db.example.com", Password: "secret", SSLMode: "disable"}, Transparencia: TransparenciaConfig{APIKey: "key"}, OTS: OTSConfig{Calendars: []string{"https://calendar.example"}}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate allowed sslmode=disable for a remote Postgres host")
	}
}

func TestValidateAllowsRemotePostgresOnlyWithTLS(t *testing.T) {
	cfg := &Config{Postgres: PostgresConfig{Host: "db.example.com", Password: "secret", SSLMode: "require"}}
	if err := cfg.ValidatePostgres(); err != nil {
		t.Fatalf("ValidatePostgres() = %v", err)
	}
}

func TestValidateAllowsUnixSocketPostgres(t *testing.T) {
	cfg := &Config{Postgres: PostgresConfig{Host: "/var/run/postgresql", Password: "secret", SSLMode: "disable"}}
	if err := cfg.ValidatePostgres(); err != nil {
		t.Fatalf("ValidatePostgres() = %v", err)
	}
}

func TestValidateAllowsEmptyTransparenciaAPIKey(t *testing.T) {
	cfg := &Config{Postgres: PostgresConfig{Host: "postgres", Password: "secret", SSLMode: "disable"}, OTS: OTSConfig{Calendars: []string{"https://calendar.example"}}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}
