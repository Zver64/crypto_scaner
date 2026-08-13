package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultHTTPAddress            = "127.0.0.1:8080"
	defaultLogLevel               = "info"
	defaultTelegramInitDataMaxAge = 15 * time.Minute
	defaultSyncWorkers            = 4
	defaultSyncRetryAttempts      = 5
	defaultShutdownTimeout        = 15 * time.Second
)

// Config is the single source of truth for environment-derived process
// settings. Sensitive fields must never be included in logs.
type Config struct {
	DatabaseURL            string
	TelegramBotToken       string
	HTTPAddress            string
	LogLevel               string
	TelegramInitDataMaxAge time.Duration
	SyncWorkers            int
	SyncRetryAttempts      int
	ShutdownTimeout        time.Duration
}

// Load reads and validates process configuration from the environment.
func Load() (Config, error) {
	var cfg Config
	var err error

	if cfg.DatabaseURL, err = required("DATABASE_URL"); err != nil {
		return Config{}, err
	}
	if _, err := pgxpool.ParseConfig(cfg.DatabaseURL); err != nil {
		return Config{}, fmt.Errorf("DATABASE_URL must be a valid PostgreSQL connection string")
	}
	if cfg.TelegramBotToken, err = required("TELEGRAM_BOT_TOKEN"); err != nil {
		return Config{}, err
	}
	cfg.HTTPAddress = valueOrDefault("HTTP_ADDRESS", defaultHTTPAddress)
	if !isHostPort(cfg.HTTPAddress) {
		return Config{}, fmt.Errorf("HTTP_ADDRESS must be a valid host:port address")
	}
	cfg.LogLevel = valueOrDefault("LOG_LEVEL", defaultLogLevel)
	if !validLogLevel(cfg.LogLevel) {
		return Config{}, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error")
	}
	if cfg.TelegramInitDataMaxAge, err = positiveDuration("TELEGRAM_INIT_DATA_MAX_AGE", defaultTelegramInitDataMaxAge); err != nil {
		return Config{}, err
	}
	if cfg.SyncWorkers, err = positiveInt("SYNC_WORKERS", defaultSyncWorkers); err != nil {
		return Config{}, err
	}
	if cfg.SyncRetryAttempts, err = positiveInt("SYNC_RETRY_ATTEMPTS", defaultSyncRetryAttempts); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = positiveDuration("SHUTDOWN_TIMEOUT", defaultShutdownTimeout); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// LoadDatabaseURL loads the only setting required by database-only commands.
// Keeping this in config preserves one source of truth for environment access.
func LoadDatabaseURL() (string, error) {
	return required("DATABASE_URL")
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
