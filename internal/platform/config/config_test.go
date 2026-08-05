package config_test

import (
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/platform/config"
)

func TestLoadUsesDocumentedDefaults(t *testing.T) {
	setRequiredEnvironment(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddress != "127.0.0.1:8080" {
		t.Errorf("HTTPAddress = %q, want %q", cfg.HTTPAddress, "127.0.0.1:8080")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.TelegramInitDataMaxAge != 15*time.Minute {
		t.Errorf("TelegramInitDataMaxAge = %v, want %v", cfg.TelegramInitDataMaxAge, 15*time.Minute)
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
}

func TestLoadRejectsMissingRequiredSettingsWithoutLeakingValues(t *testing.T) {
	required := []string{
		"DATABASE_URL",
		"TELEGRAM_BOT_TOKEN",
		"ADMIN_TELEGRAM_ID",
	}

	for _, name := range required {
		t.Run(name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv(name, "")

			_, err := config.Load()
			if err == nil || err.Error() != name+" is required" {
				t.Fatalf("Load() error = %v, want %q", err, name+" is required")
			}
		})
	}
}

func TestLoadRejectsInvalidSettingsPreciselyAndSafely(t *testing.T) {
	tests := []struct {
		name      string
		variable  string
		value     string
		wantError string
	}{
		{name: "database URL", variable: "DATABASE_URL", value: "not a connection string", wantError: "DATABASE_URL must be a valid PostgreSQL connection string"},
		{name: "administrator ID", variable: "ADMIN_TELEGRAM_ID", value: "not-an-id", wantError: "ADMIN_TELEGRAM_ID must be a positive base-10 integer"},
		{name: "HTTP address", variable: "HTTP_ADDRESS", value: "localhost", wantError: "HTTP_ADDRESS must be a valid host:port address"},
		{name: "log level", variable: "LOG_LEVEL", value: "trace", wantError: "LOG_LEVEL must be one of debug, info, warn, error"},
		{name: "init data age", variable: "TELEGRAM_INIT_DATA_MAX_AGE", value: "0s", wantError: "TELEGRAM_INIT_DATA_MAX_AGE must be a positive duration"},
		{name: "sync workers", variable: "SYNC_WORKERS", value: "0", wantError: "SYNC_WORKERS must be a positive integer"},
		{name: "retry attempts", variable: "SYNC_RETRY_ATTEMPTS", value: "many", wantError: "SYNC_RETRY_ATTEMPTS must be a positive integer"},
		{name: "shutdown timeout", variable: "SHUTDOWN_TIMEOUT", value: "later", wantError: "SHUTDOWN_TIMEOUT must be a positive duration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv(tt.variable, tt.value)

			_, err := config.Load()
			if err == nil || err.Error() != tt.wantError {
				t.Fatalf("Load() error = %v, want %q", err, tt.wantError)
			}
			if strings.Contains(err.Error(), tt.value) {
				t.Fatalf("Load() error leaks invalid value: %q", err)
			}
		})
	}
}

func TestLoadParsesOptionalSettings(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("HTTP_ADDRESS", "0.0.0.0:9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("TELEGRAM_INIT_DATA_MAX_AGE", "30m")
	t.Setenv("SYNC_WORKERS", "8")
	t.Setenv("SYNC_RETRY_ATTEMPTS", "7")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddress != "0.0.0.0:9090" ||
		cfg.LogLevel != "debug" ||
		cfg.TelegramInitDataMaxAge != 30*time.Minute ||
		cfg.SyncWorkers != 8 ||
		cfg.SyncRetryAttempts != 7 ||
		cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("Load() parsed unexpected configuration: %+v", cfg)
	}
}

func TestLoadAdminBootstrapReadsOnlyItsRequiredSettings(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://scanner:secret@localhost/scanner")
	t.Setenv("ADMIN_TELEGRAM_ID", "987654321")
	for _, name := range []string{"TELEGRAM_BOT_TOKEN"} {
		t.Setenv(name, "")
	}

	cfg, err := config.LoadAdminBootstrap()
	if err != nil {
		t.Fatalf("LoadAdminBootstrap() error = %v", err)
	}
	if cfg.DatabaseURL != "postgres://scanner:secret@localhost/scanner" || cfg.AdminTelegramID != 987654321 {
		t.Fatalf("LoadAdminBootstrap() = %#v", cfg)
	}
}

func TestLoadAdminBootstrapRejectsInvalidAdministratorID(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://scanner:secret@localhost/scanner")
	tests := []struct {
		name      string
		value     string
		wantError string
	}{
		{name: "missing", value: "", wantError: "ADMIN_TELEGRAM_ID is required"},
		{name: "not an integer", value: "not-an-id", wantError: "ADMIN_TELEGRAM_ID must be a positive base-10 integer"},
		{name: "zero", value: "0", wantError: "ADMIN_TELEGRAM_ID must be a positive base-10 integer"},
		{name: "negative", value: "-42", wantError: "ADMIN_TELEGRAM_ID must be a positive base-10 integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ADMIN_TELEGRAM_ID", tt.value)
			_, err := config.LoadAdminBootstrap()
			if err == nil || err.Error() != tt.wantError {
				t.Fatalf("LoadAdminBootstrap() error = %v, want %q", err, tt.wantError)
			}
		})
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	values := map[string]string{
		"DATABASE_URL":       "postgres://scanner:secret@localhost/scanner",
		"TELEGRAM_BOT_TOKEN": "123456:token",
		"ADMIN_TELEGRAM_ID":  "123456789",
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
	for _, key := range []string{
		"HTTP_ADDRESS",
		"LOG_LEVEL",
		"TELEGRAM_INIT_DATA_MAX_AGE",
		"SYNC_WORKERS",
		"SYNC_RETRY_ATTEMPTS",
		"SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
}
