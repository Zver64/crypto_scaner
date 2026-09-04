package config_test

import (
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/platform/config"
)

func TestLoadServerUsesDocumentedDefaults(t *testing.T) {
	setRequiredEnvironment(t)

	cfg, err := config.LoadServer()
	if err != nil {
		t.Fatalf("LoadServer() error = %v", err)
	}

	if cfg.HTTPAddress != "127.0.0.1:8080" {
		t.Errorf("HTTPAddress = %q, want %q", cfg.HTTPAddress, "127.0.0.1:8080")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.TelegramInitDataMaxAge != 24*time.Hour {
		t.Errorf("TelegramInitDataMaxAge = %v, want %v", cfg.TelegramInitDataMaxAge, 24*time.Hour)
	}
	if cfg.SyncWorkers != 4 {
		t.Errorf("SyncWorkers = %d, want 4", cfg.SyncWorkers)
	}
	if cfg.SyncRetryAttempts != 5 {
		t.Errorf("SyncRetryAttempts = %d, want 5", cfg.SyncRetryAttempts)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 15*time.Second)
	}
	if cfg.AdminTelegramID != 123456789 {
		t.Errorf("AdminTelegramID = %d, want 123456789", cfg.AdminTelegramID)
	}
}

func TestLoadServerRejectsMissingRequiredSettingsWithoutLeakingValues(t *testing.T) {
	required := []string{
		"POSTGRES_HOST",
		"POSTGRES_PORT",
		"POSTGRES_USER",
		"POSTGRES_PASSWORD",
		"POSTGRES_DB",
		"TELEGRAM_BOT_TOKEN",
		"TELEGRAM_INIT_DATA_MAX_AGE",
		"ADMIN_TELEGRAM_ID",
	}

	for _, name := range required {
		t.Run(name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv(name, "")

			_, err := config.LoadServer()
			if err == nil || err.Error() != name+" is required" {
				t.Fatalf("LoadServer() error = %v, want %q", err, name+" is required")
			}
		})
	}
}

func TestLoadServerRejectsInvalidSettingsPreciselyAndSafely(t *testing.T) {
	tests := []struct {
		name      string
		variable  string
		value     string
		wantError string
	}{
		{name: "database URL", variable: "DATABASE_URL", value: "not a connection string", wantError: "DATABASE_URL must be a valid PostgreSQL connection string"},
		{name: "HTTP address", variable: "HTTP_ADDRESS", value: "localhost", wantError: "HTTP_ADDRESS must be a valid host:port address"},
		{name: "log level", variable: "LOG_LEVEL", value: "trace", wantError: "LOG_LEVEL must be one of debug, info, warn, error"},
		{name: "init data age", variable: "TELEGRAM_INIT_DATA_MAX_AGE", value: "0s", wantError: "TELEGRAM_INIT_DATA_MAX_AGE must be a positive duration"},
		{name: "sync workers", variable: "SYNC_WORKERS", value: "0", wantError: "SYNC_WORKERS must be a positive integer"},
		{name: "retry attempts", variable: "SYNC_RETRY_ATTEMPTS", value: "many", wantError: "SYNC_RETRY_ATTEMPTS must be a positive integer"},
		{name: "shutdown timeout", variable: "SHUTDOWN_TIMEOUT", value: "later", wantError: "SHUTDOWN_TIMEOUT must be a positive duration"},
		{name: "administrator Telegram ID", variable: "ADMIN_TELEGRAM_ID", value: "not-an-id", wantError: "ADMIN_TELEGRAM_ID must be a positive base-10 integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv(tt.variable, tt.value)

			_, err := config.LoadServer()
			if err == nil || err.Error() != tt.wantError {
				t.Fatalf("LoadServer() error = %v, want %q", err, tt.wantError)
			}
			if strings.Contains(err.Error(), tt.value) {
				t.Fatalf("LoadServer() error leaks invalid value: %q", err)
			}
		})
	}
}

func TestLoadServerParsesOptionalSettings(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("HTTP_ADDRESS", "0.0.0.0:9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("TELEGRAM_INIT_DATA_MAX_AGE", "30m")
	t.Setenv("SYNC_WORKERS", "8")
	t.Setenv("SYNC_RETRY_ATTEMPTS", "7")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")

	cfg, err := config.LoadServer()
	if err != nil {
		t.Fatalf("LoadServer() error = %v", err)
	}

	if cfg.HTTPAddress != "0.0.0.0:9090" ||
		cfg.LogLevel != "debug" ||
		cfg.TelegramInitDataMaxAge != 30*time.Minute ||
		cfg.SyncWorkers != 8 ||
		cfg.SyncRetryAttempts != 7 ||
		cfg.ShutdownTimeout != 3*time.Second ||
		cfg.AdminTelegramID != 123456789 {
		t.Fatalf("LoadServer() parsed unexpected configuration: %+v", cfg)
	}
}

func TestLoadServerUsesAdministratorConfiguration(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("ADMIN_TELEGRAM_ID", "not-a-telegram-id")

	if _, err := config.LoadServer(); err == nil || err.Error() != "ADMIN_TELEGRAM_ID must be a positive base-10 integer" {
		t.Fatalf("LoadServer() error = %v", err)
	}
}
func TestLoadBootstrap(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://scanner:secret@127.0.0.1:5432/scanner?sslmode=disable")
	t.Setenv("ADMIN_TELEGRAM_ID", "123456789")

	cfg, err := config.LoadBootstrap()
	if err != nil {
		t.Fatalf("LoadBootstrap() error = %v", err)
	}
	if cfg.AdminTelegramID != 123456789 {
		t.Fatalf("AdminTelegramID = %d", cfg.AdminTelegramID)
	}
}

func TestLoadDatabaseURLPrefersExplicitURL(t *testing.T) {
	explicit := "postgres://production:secret@database.example/production?sslmode=require"
	t.Setenv("DATABASE_URL", explicit)
	t.Setenv("POSTGRES_HOST", "127.0.0.1")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_USER", "local")
	t.Setenv("POSTGRES_PASSWORD", "local-secret")
	t.Setenv("POSTGRES_DB", "local")

	got, err := config.LoadDatabaseURL()
	if err != nil {
		t.Fatalf("LoadDatabaseURL() error = %v", err)
	}
	if got != explicit {
		t.Fatalf("LoadDatabaseURL() = %q, want explicit DATABASE_URL", got)
	}
}

func TestLoadDatabaseURLBuildsEscapedURLFromPostgresSettings(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_HOST", "127.0.0.1")
	t.Setenv("POSTGRES_PORT", "55432")
	t.Setenv("POSTGRES_USER", "scan@ner")
	t.Setenv("POSTGRES_PASSWORD", "p:a/ss?#%")
	t.Setenv("POSTGRES_DB", "scanner/local")

	got, err := config.LoadDatabaseURL()
	if err != nil {
		t.Fatalf("LoadDatabaseURL() error = %v", err)
	}
	want := "postgres://scan%40ner:p%3Aa%2Fss%3F%23%25@127.0.0.1:55432/scanner%2Flocal?sslmode=disable"
	if got != want {
		t.Fatalf("LoadDatabaseURL() = %q, want %q", got, want)
	}
}

func TestLoadBootstrapRejectsInvalidTelegramID(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://scanner:secret@127.0.0.1:5432/scanner?sslmode=disable")
	t.Setenv("ADMIN_TELEGRAM_ID", "not-an-id")

	_, err := config.LoadBootstrap()
	if err == nil || err.Error() != "ADMIN_TELEGRAM_ID must be a positive base-10 integer" {
		t.Fatalf("LoadBootstrap() error = %v", err)
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	values := map[string]string{
		"DATABASE_URL":               "",
		"POSTGRES_HOST":              "127.0.0.1",
		"POSTGRES_PORT":              "5432",
		"POSTGRES_USER":              "scanner",
		"POSTGRES_PASSWORD":          "secret",
		"POSTGRES_DB":                "scanner",
		"TELEGRAM_BOT_TOKEN":         "123456:token",
		"TELEGRAM_INIT_DATA_MAX_AGE": "24h",
		"ADMIN_TELEGRAM_ID":          "123456789",
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
	for _, key := range []string{
		"HTTP_ADDRESS",
		"LOG_LEVEL",
		"SYNC_WORKERS",
		"SYNC_RETRY_ATTEMPTS",
		"SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
}
