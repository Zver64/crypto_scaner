package logging_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"crypto-scanner/internal/platform/logging"
)

// A key warns when it starts failing, logs repeats at INFO, and warns again
// only after it recovered; keys are independent.
func TestFailuresWarnOncePerFailure(t *testing.T) {
	var output bytes.Buffer
	failures := logging.NewFailures[string](slog.New(slog.NewTextHandler(&output, nil)))
	ctx := context.Background()

	failures.Warn(ctx, "a", "failed")
	failures.Warn(ctx, "a", "failed")
	failures.Warn(ctx, "b", "failed")
	failures.Recover(ctx, "a", "recovered")
	failures.Recover(ctx, "a", "recovered")
	failures.Warn(ctx, "a", "failed")

	var levels []string
	for line := range strings.Lines(output.String()) {
		fields := strings.Fields(line)
		levels = append(levels, strings.TrimPrefix(fields[1], "level=")+" "+strings.TrimPrefix(fields[2], "msg="))
	}
	want := []string{"WARN failed", "INFO failed", "WARN failed", "INFO recovered", "WARN failed"}
	if strings.Join(levels, ",") != strings.Join(want, ",") {
		t.Fatalf("records = %q, want %q", levels, want)
	}
}
