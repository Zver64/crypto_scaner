package logging_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"crypto-scanner/internal/platform/logging"
)

func TestNewEmitsJSONAtTheConfiguredLevel(t *testing.T) {
	var output bytes.Buffer
	logger := logging.New(&output, "warn")

	logger.Info("not emitted")
	logger.Warn("sync delayed", "module", "market", "retry_count", 2)

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("log lines = %d, want 1; output = %q", len(lines), output.String())
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log output is not JSON: %v", err)
	}
	if entry["level"] != "WARN" || entry["msg"] != "sync delayed" || entry["module"] != "market" || entry["retry_count"] != float64(2) {
		t.Fatalf("unexpected log entry: %#v", entry)
	}
}

func TestNewRedactsSensitiveKeysAndConfiguredSecrets(t *testing.T) {
	var output bytes.Buffer
	const secret = "postgres://scanner:password@localhost/scanner"
	logger := logging.New(&output, "info", secret, "bot-token")

	logger.Error(
		"connection failed: "+secret,
		"database_url", secret,
		"authorization", "tma raw-init-data",
		"error", errors.New("could not use bot-token"),
		slog.Group("telegram", "init_data", "raw-init-data", "update", "full-update"),
	)

	logged := output.String()
	for _, forbidden := range []string{secret, "password", "bot-token", "raw-init-data", "full-update"} {
		if strings.Contains(logged, forbidden) {
			t.Errorf("log contains sensitive value %q: %s", forbidden, logged)
		}
	}
	if !strings.Contains(logged, "[REDACTED]") {
		t.Fatalf("log does not contain redaction marker: %s", logged)
	}
}

func TestNewRedactsOpaqueValuesAndSensitiveKeyVariants(t *testing.T) {
	type requestPayload struct {
		BotToken string `json:"bot_token"`
		InitData string `json:"init_data"`
	}

	var output bytes.Buffer
	logger := logging.New(&output, "info", "configured-bot-token")
	logger.Info("received request",
		"payload", requestPayload{BotToken: "configured-bot-token", InitData: "raw-telegram-init-data"},
		"authorization_header", "tma authorization-value",
		"raw_telegram_init_data", "raw-init-value",
		"database-url", "postgres://user:password@db/scanner",
	)

	logged := output.String()
	for _, forbidden := range []string{
		"configured-bot-token",
		"raw-telegram-init-data",
		"authorization-value",
		"raw-init-value",
		"password",
	} {
		if strings.Contains(logged, forbidden) {
			t.Errorf("log contains sensitive value %q: %s", forbidden, logged)
		}
	}
}
