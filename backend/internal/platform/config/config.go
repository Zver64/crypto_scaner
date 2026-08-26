package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultHTTPAddress       = "127.0.0.1:8080"
	defaultLogLevel          = "info"
	defaultSyncWorkers       = 4
	defaultSyncRetryAttempts = 5
	defaultShutdownTimeout   = 15 * time.Second
)

// ServerConfig is the single source of truth for environment-derived server
// settings. Sensitive fields must never be included in logs.
type ServerConfig struct {
	DatabaseURL            string
	TelegramBotToken       string
	HTTPAddress            string
	LogLevel               string
	TelegramInitDataMaxAge time.Duration
	SyncWorkers            int
	SyncRetryAttempts      int
	ShutdownTimeout        time.Duration
	CoinGeckoDemoAPIKey    string
}

// BootstrapConfig contains the settings used only by the explicit
// administrator bootstrap command.
type BootstrapConfig struct {
	DatabaseURL     string
	AdminTelegramID int64
}

// LoadServer reads and validates server configuration from the environment.
func LoadServer() (ServerConfig, error) {
	var cfg ServerConfig
	var err error

	if cfg.DatabaseURL, err = LoadDatabaseURL(); err != nil {
		return ServerConfig{}, err
	}
	if cfg.TelegramBotToken, err = required("TELEGRAM_BOT_TOKEN"); err != nil {
		return ServerConfig{}, err
	}
	cfg.HTTPAddress = valueOrDefault("HTTP_ADDRESS", defaultHTTPAddress)
	cfg.CoinGeckoDemoAPIKey = os.Getenv("COINGECKO_DEMO_API_KEY")
	if !isHostPort(cfg.HTTPAddress) {
		return ServerConfig{}, fmt.Errorf("HTTP_ADDRESS must be a valid host:port address")
	}
	cfg.LogLevel = valueOrDefault("LOG_LEVEL", defaultLogLevel)
	if !validLogLevel(cfg.LogLevel) {
		return ServerConfig{}, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error")
	}
	if cfg.TelegramInitDataMaxAge, err = requiredPositiveDuration("TELEGRAM_INIT_DATA_MAX_AGE"); err != nil {
		return ServerConfig{}, err
	}
	if cfg.SyncWorkers, err = positiveInt("SYNC_WORKERS", defaultSyncWorkers); err != nil {
		return ServerConfig{}, err
	}
	if cfg.SyncRetryAttempts, err = positiveInt("SYNC_RETRY_ATTEMPTS", defaultSyncRetryAttempts); err != nil {
		return ServerConfig{}, err
	}
	if cfg.ShutdownTimeout, err = positiveDuration("SHUTDOWN_TIMEOUT", defaultShutdownTimeout); err != nil {
		return ServerConfig{}, err
	}

	return cfg, nil
}

// LoadDatabaseURL loads the only setting required by database-only commands.
// Keeping this in config preserves one source of truth for environment access.
func LoadDatabaseURL() (string, error) {
	if databaseURL := os.Getenv("DATABASE_URL"); strings.TrimSpace(databaseURL) != "" {
		return validateDatabaseURL(databaseURL)
	}

	host, err := required("POSTGRES_HOST")
	if err != nil {
		return "", err
	}
	port, err := required("POSTGRES_PORT")
	if err != nil {
		return "", err
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber <= 0 || portNumber > 65535 {
		return "", fmt.Errorf("POSTGRES_PORT must be an integer between 1 and 65535")
	}
	user, err := required("POSTGRES_USER")
	if err != nil {
		return "", err
	}
	password, err := required("POSTGRES_PASSWORD")
	if err != nil {
		return "", err
	}
	database, err := required("POSTGRES_DB")
	if err != nil {
		return "", err
	}

	databaseURL := (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + database,
		RawPath:  "/" + url.PathEscape(database),
		RawQuery: url.Values{"sslmode": {"disable"}}.Encode(),
	}).String()
	return validateDatabaseURL(databaseURL)
}

func validateDatabaseURL(databaseURL string) (string, error) {
	if _, err := pgxpool.ParseConfig(databaseURL); err != nil {
		return "", fmt.Errorf("DATABASE_URL must be a valid PostgreSQL connection string")
	}
	return databaseURL, nil
}

// LoadBootstrap reads configuration for the explicit administrator bootstrap.
func LoadBootstrap() (BootstrapConfig, error) {
	databaseURL, err := LoadDatabaseURL()
	if err != nil {
		return BootstrapConfig{}, err
	}
	adminTelegramID, err := positiveInt64("ADMIN_TELEGRAM_ID")
	if err != nil {
		return BootstrapConfig{}, err
	}
	return BootstrapConfig{DatabaseURL: databaseURL, AdminTelegramID: adminTelegramID}, nil
}

func required(name string) (string, error) {
	value := os.Getenv(name)
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func positiveDuration(name string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return duration, nil
}

func requiredPositiveDuration(name string) (time.Duration, error) {
	value, err := required(name)
	if err != nil {
		return 0, err
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return duration, nil
}

func positiveInt(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return number, nil
}

func positiveInt64(name string) (int64, error) {
	value, err := required(name)
	if err != nil {
		return 0, err
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("%s must be a positive base-10 integer", name)
	}
	return number, nil
}

func isHostPort(value string) bool {
	_, port, err := net.SplitHostPort(value)
	if err != nil {
		return false
	}
	number, err := strconv.Atoi(port)
	return err == nil && number > 0 && number <= 65535
}

func validLogLevel(value string) bool {
	switch value {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

func valueOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
