package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"crypto-scanner/internal/auth"
)

func TestInternalErrorDescribesTheRequest(t *testing.T) {
	var output bytes.Buffer
	api := &api{logger: slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))}
	ctx := context.WithValue(context.Background(), requestIDContextKey{}, "request-1")
	ctx = context.WithValue(ctx, requestInfoContextKey{}, requestInfo{method: "GET", path: "/api/v1/analysis/instruments/BTCUSDT", started: time.Now()})
	ctx = context.WithValue(ctx, userContextKey{}, auth.User{TelegramID: 42})

	api.internalError(ctx, "analyze_instrument", errors.New("select active instruments: connection reset"))

	record := decodeLogRecord(t, output.Bytes())
	for key, want := range map[string]any{
		"level":       "ERROR",
		"msg":         "HTTP operation failed",
		"operation":   "analyze_instrument",
		"request_id":  "request-1",
		"method":      "GET",
		"path":        "/api/v1/analysis/instruments/BTCUSDT",
		"telegram_id": float64(42),
	} {
		if record[key] != want {
			t.Errorf("%s = %v, want %v", key, record[key], want)
		}
	}
	if _, ok := record["duration"]; !ok {
		t.Error("duration is missing")
	}
}

func TestInternalErrorLogsAClientCancellationAsInfo(t *testing.T) {
	var output bytes.Buffer
	api := &api{logger: slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	api.internalError(ctx, "analyze_instrument", fmt.Errorf("select active instruments: %w", context.Canceled))

	record := decodeLogRecord(t, output.Bytes())
	if record["level"] != "INFO" || record["msg"] != "HTTP operation canceled by the client" {
		t.Errorf("record = %v, want an INFO cancellation", record)
	}
}

func TestInternalErrorKeepsACancellationOfALiveRequestAsError(t *testing.T) {
	var output bytes.Buffer
	api := &api{logger: slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))}

	api.internalError(context.Background(), "analyze_instrument", context.Canceled)

	if record := decodeLogRecord(t, output.Bytes()); record["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", record["level"])
	}
}

func decodeLogRecord(t *testing.T, output []byte) map[string]any {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal(output, &record); err != nil {
		t.Fatalf("decode log record %q: %v", output, err)
	}
	return record
}
