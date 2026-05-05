package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPPort         = 8080
	defaultPostgresPort     = 5432
	defaultPostgresSSLMode  = "disable"
	defaultPostgresTimeZone = "UTC"
	defaultSyncSchedule     = "1 0 * * *"
	defaultCNBBaseURL       = "https://www.cnb.cz/en/financial_markets/foreign_exchange_market/exchange_rate_fixing"
	defaultHistoryStartDate = "2019-01-01"
	defaultHistoryEndDate   = "2019-12-31"
	dateLayout              = "2006-01-02"
)

type Config struct {
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Sync     SyncConfig
	CNB      CNBConfig
}

type HTTPConfig struct {
	Port int
}

type PostgresConfig struct {
	DSN      string
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	TimeZone string
}

type SyncConfig struct {
	Schedule         string
	SchedulerEnabled bool
	Currencies       []string
	HistoryStartDate time.Time
	HistoryEndDate   time.Time
}

type CNBConfig struct {
	BaseURL string
}

func LoadFromEnv() (Config, error) {
	return loadFromLookupEnv(os.LookupEnv)
}

func loadFromLookupEnv(lookup func(string) (string, bool)) (Config, error) {
	httpPort, err := envInt(lookup, "HTTP_PORT", defaultHTTPPort)
	if err != nil {
		return Config{}, err
	}

	postgresPort, err := envInt(lookup, "POSTGRES_PORT", defaultPostgresPort)
	if err != nil {
		return Config{}, err
	}

	schedulerEnabled, err := envBool(lookup, "SCHEDULER_ENABLED", false)
	if err != nil {
		return Config{}, err
	}

	historyStartDate, err := envDate(lookup, "SYNC_HISTORY_START_DATE", defaultHistoryStartDate)
	if err != nil {
		return Config{}, err
	}

	historyEndDate, err := envDate(lookup, "SYNC_HISTORY_END_DATE", defaultHistoryEndDate)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTP: HTTPConfig{
			Port: httpPort,
		},
		Postgres: PostgresConfig{
			DSN:      strings.TrimSpace(envString(lookup, "POSTGRES_DSN", "")),
			Host:     strings.TrimSpace(envString(lookup, "POSTGRES_HOST", "")),
			Port:     postgresPort,
			User:     strings.TrimSpace(envString(lookup, "POSTGRES_USER", "")),
			Password: envString(lookup, "POSTGRES_PASSWORD", ""),
			DBName:   strings.TrimSpace(envString(lookup, "POSTGRES_DB", "")),
			SSLMode:  strings.TrimSpace(envString(lookup, "POSTGRES_SSLMODE", defaultPostgresSSLMode)),
			TimeZone: strings.TrimSpace(envString(lookup, "POSTGRES_TIMEZONE", defaultPostgresTimeZone)),
		},
		Sync: SyncConfig{
			Schedule:         strings.TrimSpace(envString(lookup, "SYNC_SCHEDULE", defaultSyncSchedule)),
			SchedulerEnabled: schedulerEnabled,
			Currencies:       parseCurrencies(envString(lookup, "SYNC_CURRENCIES", "")),
			HistoryStartDate: historyStartDate,
			HistoryEndDate:   historyEndDate,
		},
		CNB: CNBConfig{
			BaseURL: strings.TrimSpace(envString(lookup, "CNB_BASE_URL", defaultCNBBaseURL)),
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.HTTP.Port <= 0 {
		return fmt.Errorf("http port must be positive")
	}

	if err := c.Postgres.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(c.Sync.Schedule) == "" {
		return fmt.Errorf("sync schedule must not be empty")
	}

	if len(c.Sync.Currencies) == 0 {
		return fmt.Errorf("sync currencies must not be empty")
	}

	for _, currency := range c.Sync.Currencies {
		if !isValidCurrencyCode(currency) {
			return fmt.Errorf("invalid currency code %q", currency)
		}
	}

	if c.Sync.HistoryStartDate.IsZero() || c.Sync.HistoryEndDate.IsZero() {
		return fmt.Errorf("sync history period must be set")
	}

	if c.Sync.HistoryStartDate.After(c.Sync.HistoryEndDate) {
		return fmt.Errorf("sync history start date must be before or equal to end date")
	}

	if strings.TrimSpace(c.CNB.BaseURL) == "" {
		return fmt.Errorf("cnb base url must not be empty")
	}

	return nil
}

func (c Config) PostgresDSN() string {
	return c.Postgres.DSNString()
}

func (c PostgresConfig) Validate() error {
	if c.DSN != "" {
		return nil
	}

	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("postgres host must not be empty")
	}

	if c.Port <= 0 {
		return fmt.Errorf("postgres port must be positive")
	}

	if strings.TrimSpace(c.User) == "" {
		return fmt.Errorf("postgres user must not be empty")
	}

	if strings.TrimSpace(c.DBName) == "" {
		return fmt.Errorf("postgres db name must not be empty")
	}

	return nil
}

func (c PostgresConfig) DSNString() string {
	if strings.TrimSpace(c.DSN) != "" {
		return strings.TrimSpace(c.DSN)
	}

	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = defaultPostgresSSLMode
	}

	timeZone := c.TimeZone
	if timeZone == "" {
		timeZone = defaultPostgresTimeZone
	}

	port := c.Port
	if port == 0 {
		port = defaultPostgresPort
	}

	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		c.Host,
		port,
		c.User,
		c.Password,
		c.DBName,
		sslMode,
		timeZone,
	)
}

func envString(lookup func(string) (string, bool), key, fallback string) string {
	value, ok := lookup(key)
	if !ok {
		return fallback
	}

	return value
}

func envInt(lookup func(string) (string, bool), key string, fallback int) (int, error) {
	value := strings.TrimSpace(envString(lookup, key, ""))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}

	return parsed, nil
}

func envBool(lookup func(string) (string, bool), key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(envString(lookup, key, ""))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}

	return parsed, nil
}

func envDate(lookup func(string) (string, bool), key, fallback string) (time.Time, error) {
	value := strings.TrimSpace(envString(lookup, key, fallback))
	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be in %s format: %w", key, dateLayout, err)
	}

	return parsed, nil
}

func parseCurrencies(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	currencies := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		code := strings.ToUpper(strings.TrimSpace(part))
		if code == "" {
			continue
		}

		if _, exists := seen[code]; exists {
			continue
		}

		seen[code] = struct{}{}
		currencies = append(currencies, code)
	}

	return currencies
}

func isValidCurrencyCode(code string) bool {
	if len(code) != 3 {
		return false
	}

	for _, char := range code {
		if char < 'A' || char > 'Z' {
			return false
		}
	}

	return true
}
