package opsnotify

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/telegramhtml"
)

// Long multi-line texts are joined into one line each and capped, so the
// administrator gets the message and its attributes within Telegram's limit,
// without a cut tag or entity.
func TestFormatCapsLongLines(t *testing.T) {
	record := slog.NewRecord(time.Now(), slog.LevelWarn, strings.Repeat("refresh\nfailed ", 400), 0)
	record.AddAttrs(slog.String("module", "coin_metadata"), slog.String("error", "upstream status 502:"+strings.Repeat("<html>\nbad gateway\n", 400)))
	got := format(record, 0)
	lines := strings.Split(got, "\n")
	if len(got) > telegramhtml.MaxLength || len(lines) != 3 || lines[0] != "<b>⚠️ WARN · coin_metadata</b>" ||
		!strings.HasPrefix(lines[1], "refresh failed refresh") || !strings.HasSuffix(lines[1], "…") ||
		!strings.HasPrefix(lines[2], "error: <code>upstream status 502:&lt;html&gt;") || !strings.HasSuffix(lines[2], "bad gateway …</code>") {
		t.Fatalf("format() = %q", got)
	}
}

// Shutdown sends the queued records and the repeats not yet summarized.
func TestFlushSendsQueuedRecordsAndPendingSummaries(t *testing.T) {
	queue := NewQueue()
	sender := &recordingSender{}
	notifier, err := New(queue, sender, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		queue.Forward(slog.NewRecord(time.Now(), slog.LevelWarn, "stream stopped", 0))
	}

	notifier.Flush(t.Context())

	if len(sender.texts) != 2 || !strings.Contains(sender.texts[1], "Repeated 2 times") {
		t.Fatalf("sent %q, want the record and its summary", sender.texts)
	}
}

// Shutdown sends the failure before the warnings queued ahead of it.
func TestFlushSendsErrorsFirst(t *testing.T) {
	queue := NewQueue()
	sender := &recordingSender{}
	notifier, err := New(queue, sender, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	queue.Forward(slog.NewRecord(time.Now(), slog.LevelWarn, "stream stopped", 0))
	queue.Forward(slog.NewRecord(time.Now(), slog.LevelError, "application failed", 0))

	notifier.Flush(t.Context())

	if len(sender.texts) != 2 || !strings.Contains(sender.texts[0], "application failed") {
		t.Fatalf("sent %q, want the error first", sender.texts)
	}
}

// A record whose delivery stopping Run aborts is delivered by Flush.
func TestFlushSendsRecordAbortedByStop(t *testing.T) {
	queue := NewQueue()
	sender := &blockingSender{started: make(chan struct{}, 1), block: true}
	notifier, err := New(queue, sender, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = notifier.Run(ctx)
	}()
	queue.Forward(slog.NewRecord(time.Now(), slog.LevelError, "application failed", 0))
	<-sender.started
	stop()
	<-done

	sender.block = false
	notifier.Flush(t.Context())

	if len(sender.texts) != 1 || !strings.Contains(sender.texts[0], "application failed") {
		t.Fatalf("sent %q, want the aborted record", sender.texts)
	}
}

// blockingSender blocks while block is set until ctx ends, failing the
// delivery, and records the deliveries made otherwise.
type blockingSender struct {
	recordingSender
	started chan struct{}
	block   bool
}

func (sender *blockingSender) SendAdministratorMessage(ctx context.Context, text string) error {
	if sender.block {
		sender.started <- struct{}{}
		<-ctx.Done()
		return ctx.Err()
	}
	return sender.recordingSender.SendAdministratorMessage(ctx, text)
}

type recordingSender struct{ texts []string }

func (sender *recordingSender) SendAdministratorMessage(_ context.Context, text string) error {
	sender.texts = append(sender.texts, text)
	return nil
}
