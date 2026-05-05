package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvUsesDefaults(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_USER", "postgres")
	t.Setenv("POSTGRES_DB", "exchange_rates")
	t.Setenv("SYNC_CURRENCIES", "usd, eur")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.HTTP.Port != defaultHTTPPort {
		t.Fatalf("HTTP.Port = %d, want %d", cfg.HTTP.Port, defaultHTTPPort)
	}

	if cfg.Postgres.Port != defaultPostgresPort {
		t.Fatalf("Postgres.Port = %d, want %d", cfg.Postgres.Port, defaultPostgresPort)
	}

	if cfg.Postgres.SSLMode != defaultPostgresSSLMode {
		t.Fatalf("Postgres.SSLMode = %q, want %q", cfg.Postgres.SSLMode, defaultPostgresSSLMode)
	}

	if cfg.Postgres.TimeZone != defaultPostgresTimeZone {
		t.Fatalf("Postgres.TimeZone = %q, want %q", cfg.Postgres.TimeZone, defaultPostgresTimeZone)
	}

	if cfg.Sync.Schedule != defaultSyncSchedule {
		t.Fatalf("Sync.Schedule = %q, want %q", cfg.Sync.Schedule, defaultSyncSchedule)
	}

	if cfg.Sync.SchedulerEnabled {
		t.Fatal("Sync.SchedulerEnabled = true, want false by default")
	}

	if cfg.CNB.BaseURL != defaultCNBBaseURL {
		t.Fatalf("CNB.BaseURL = %q, want %q", cfg.CNB.BaseURL, defaultCNBBaseURL)
	}

	wantStart := mustParseDate(t, defaultHistoryStartDate)
	wantEnd := mustParseDate(t, defaultHistoryEndDate)

	if !cfg.Sync.HistoryStartDate.Equal(wantStart) {
		t.Fatalf("HistoryStartDate = %v, want %v", cfg.Sync.HistoryStartDate, wantStart)
	}

	if !cfg.Sync.HistoryEndDate.Equal(wantEnd) {
		t.Fatalf("HistoryEndDate = %v, want %v", cfg.Sync.HistoryEndDate, wantEnd)
	}

	wantCurrencies := []string{"USD", "EUR"}
	assertCurrenciesEqual(t, cfg.Sync.Currencies, wantCurrencies)
}

func TestLoadFromEnvReadsEnvironmentValues(t *testing.T) {
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@db:5432/exchange?sslmode=disable")
	t.Setenv("POSTGRES_HOST", "ignored-host")
	t.Setenv("POSTGRES_PORT", "6432")
	t.Setenv("POSTGRES_USER", "ignored-user")
	t.Setenv("POSTGRES_PASSWORD", "ignored-password")
	t.Setenv("POSTGRES_DB", "ignored-db")
	t.Setenv("POSTGRES_SSLMODE", "require")
	t.Setenv("POSTGRES_TIMEZONE", "Europe/Prague")
	t.Setenv("SYNC_SCHEDULE", "*/15 * * * *")
	t.Setenv("SCHEDULER_ENABLED", "true")
	t.Setenv("SYNC_CURRENCIES", "usd, eur, usd, gbp")
	t.Setenv("SYNC_HISTORY_START_DATE", "2024-01-01")
	t.Setenv("SYNC_HISTORY_END_DATE", "2024-03-31")
	t.Setenv("CNB_BASE_URL", "https://example.test/cnb")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.HTTP.Port != 9090 {
		t.Fatalf("HTTP.Port = %d, want 9090", cfg.HTTP.Port)
	}

	if cfg.Postgres.DSN != "postgres://user:pass@db:5432/exchange?sslmode=disable" {
		t.Fatalf("Postgres.DSN = %q", cfg.Postgres.DSN)
	}

	if cfg.Postgres.Port != 6432 {
		t.Fatalf("Postgres.Port = %d, want 6432", cfg.Postgres.Port)
	}

	if cfg.Postgres.SSLMode != "require" {
		t.Fatalf("Postgres.SSLMode = %q, want require", cfg.Postgres.SSLMode)
	}

	if cfg.Postgres.TimeZone != "Europe/Prague" {
		t.Fatalf("Postgres.TimeZone = %q, want Europe/Prague", cfg.Postgres.TimeZone)
	}

	if cfg.Sync.Schedule != "*/15 * * * *" {
		t.Fatalf("Sync.Schedule = %q, want */15 * * * *", cfg.Sync.Schedule)
	}

	if !cfg.Sync.SchedulerEnabled {
		t.Fatal("Sync.SchedulerEnabled = false, want true")
	}

	assertCurrenciesEqual(t, cfg.Sync.Currencies, []string{"USD", "EUR", "GBP"})

	if !cfg.Sync.HistoryStartDate.Equal(mustParseDate(t, "2024-01-01")) {
		t.Fatalf("HistoryStartDate = %v", cfg.Sync.HistoryStartDate)
	}

	if !cfg.Sync.HistoryEndDate.Equal(mustParseDate(t, "2024-03-31")) {
		t.Fatalf("HistoryEndDate = %v", cfg.Sync.HistoryEndDate)
	}

	if cfg.CNB.BaseURL != "https://example.test/cnb" {
		t.Fatalf("CNB.BaseURL = %q", cfg.CNB.BaseURL)
	}
}

func TestPostgresDSNBuildsFromFields(t *testing.T) {
	cfg := Config{
		Postgres: PostgresConfig{
			Host:     "db",
			Port:     6432,
			User:     "rates_user",
			Password: "secret",
			DBName:   "rates",
			SSLMode:  "require",
			TimeZone: "Europe/Prague",
		},
	}

	got := cfg.PostgresDSN()
	want := "host=db port=6432 user=rates_user password=secret dbname=rates sslmode=require TimeZone=Europe/Prague"
	if got != want {
		t.Fatalf("PostgresDSN() = %q, want %q", got, want)
	}
}

func TestPostgresDSNReturnsConfiguredDSN(t *testing.T) {
	cfg := Config{
		Postgres: PostgresConfig{
			DSN: "postgres://user:pass@db:5432/exchange?sslmode=disable",
		},
	}

	if got := cfg.PostgresDSN(); got != cfg.Postgres.DSN {
		t.Fatalf("PostgresDSN() = %q, want %q", got, cfg.Postgres.DSN)
	}
}

func TestLoadFromEnvRejectsInvalidCurrency(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_USER", "postgres")
	t.Setenv("POSTGRES_DB", "exchange_rates")
	t.Setenv("SYNC_CURRENCIES", "USD,EURO")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want validation error")
	}
}

func TestLoadFromEnvRejectsInvalidDateRange(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_USER", "postgres")
	t.Setenv("POSTGRES_DB", "exchange_rates")
	t.Setenv("SYNC_CURRENCIES", "USD,EUR")
	t.Setenv("SYNC_HISTORY_START_DATE", "2024-05-01")
	t.Setenv("SYNC_HISTORY_END_DATE", "2024-04-01")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want validation error")
	}
}

func TestLoadFromEnvRejectsInvalidDateFormat(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_USER", "postgres")
	t.Setenv("POSTGRES_DB", "exchange_rates")
	t.Setenv("SYNC_CURRENCIES", "USD,EUR")
	t.Setenv("SYNC_HISTORY_START_DATE", "01-05-2024")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want parse error")
	}
}

func mustParseDate(t *testing.T, value string) time.Time {
	t.Helper()

	date, err := time.Parse(dateLayout, value)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v", value, err)
	}

	return date
}

func assertCurrenciesEqual(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("currencies length = %d, want %d; got = %v", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("currencies[%d] = %q, want %q; got = %v", i, got[i], want[i], got)
		}
	}
}
