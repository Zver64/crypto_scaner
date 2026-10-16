package opsnotify

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

// A long multi-line value is joined into one line, so cutting the message
// at a whole line never leaves a tag open.
func TestFormatCutsLongValuesAtWholeLines(t *testing.T) {
	record := slog.NewRecord(time.Now(), slog.LevelWarn, "coin metadata refresh failed", 0)
	record.AddAttrs(slog.String("module", "coin_metadata"), slog.String("error", strings.Repeat("<html>\nbad gateway\n", 400)))
	got := format(record, 0)
	if got != "<b>⚠️ WARN · coin_metadata</b>\ncoin metadata refresh failed\n…" {
		t.Fatalf("format() = %q", got)
	}
}
