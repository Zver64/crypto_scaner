// Package opsnotify delivers the process's warning and error log records to
// the administrator, collapsing repeats into one hourly summary.
package opsnotify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"
)

const (
	queueSize = 256
	// repeatWindow is how long repeats of a delivered record are only counted.
	repeatWindow = time.Hour
	flushPeriod  = time.Minute
	// messageLimit is the Telegram message length limit in characters.
	messageLimit = 4096
)

// Queue buffers forwarded log records until the Notifier runs.
type Queue struct {
	records chan slog.Record
	dropped atomic.Int64
}

func NewQueue() *Queue {
	return &Queue{records: make(chan slog.Record, queueSize)}
}

// Forward never blocks: a record that does not fit is counted and dropped.
func (q *Queue) Forward(record slog.Record) {
	select {
	case q.records <- record:
	default:
		q.dropped.Add(1)
	}
}

// Sender delivers one message to the administrator.
type Sender interface {
	SendAdministratorMessage(ctx context.Context, text string) error
}

type group struct {
	since   time.Time
	repeats int
	latest  slog.Record
}

// Notifier sends the first record of every level, module, and message at once
// and its repeats as one summary per hour.
type Notifier struct {
	queue  *Queue
	sender Sender
	// logger must not forward to queue, or a failed delivery would be queued.
	logger *slog.Logger
	groups map[string]*group
}

func New(queue *Queue, sender Sender, logger *slog.Logger) (*Notifier, error) {
	if queue == nil || sender == nil || logger == nil {
		return nil, fmt.Errorf("invalid operations notifier")
	}
	return &Notifier{queue: queue, sender: sender, logger: logger.With("module", "ops_notify"), groups: map[string]*group{}}, nil
}

func (n *Notifier) Run(ctx context.Context) error {
	ticker := time.NewTicker(flushPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case record := <-n.queue.records:
			n.receive(ctx, record)
		case now := <-ticker.C:
			n.flush(ctx, now)
		}
	}
}

// Flush handles the records still queued. Call it only after Run returned.
func (n *Notifier) Flush(ctx context.Context) {
	for {
		select {
		case record := <-n.queue.records:
			n.receive(ctx, record)
		default:
			return
		}
	}
}

func (n *Notifier) receive(ctx context.Context, record slog.Record) {
	key := record.Level.String() + "\x00" + module(record) + "\x00" + record.Message
	if existing, ok := n.groups[key]; ok {
		existing.repeats++
		existing.latest = record
		return
	}
	n.groups[key] = &group{since: time.Now(), latest: record}
	n.send(ctx, format(record, 0))
}

func (n *Notifier) flush(ctx context.Context, now time.Time) {
	for key, existing := range n.groups {
		if now.Sub(existing.since) < repeatWindow {
			continue
		}
		if existing.repeats == 0 {
			delete(n.groups, key)
			continue
		}
		n.send(ctx, format(existing.latest, existing.repeats))
		existing.since, existing.repeats = now, 0
	}
	if dropped := n.queue.dropped.Swap(0); dropped > 0 {
		n.send(ctx, fmt.Sprintf("⚠️ %d log records were dropped because the notification queue was full", dropped))
	}
}

func (n *Notifier) send(ctx context.Context, text string) {
	if err := n.sender.SendAdministratorMessage(ctx, text); err != nil && ctx.Err() == nil {
		n.logger.WarnContext(ctx, "administrator notification failed", "error", err)
	}
}

func module(record slog.Record) string {
	result := ""
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "module" {
			result = attr.Value.String()
			return false
		}
		return true
	})
	return result
}

func format(record slog.Record, repeats int) string {
	icon := "⚠️"
	if record.Level >= slog.LevelError {
		icon = "🛑"
	}
	var text strings.Builder
	text.WriteString(icon + " " + record.Level.String())
	if name := module(record); name != "" {
		text.WriteString(" · " + name)
	}
	text.WriteString("\n" + record.Message)
	if repeats > 0 {
		fmt.Fprintf(&text, "\nповторилось %d раз за час, последний случай:", repeats)
	}
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key != "module" {
			text.WriteString("\n" + attr.Key + ": " + attr.Value.String())
		}
		return true
	})
	if runes := []rune(text.String()); len(runes) > messageLimit {
		return string(runes[:messageLimit-1]) + "…"
	}
	return text.String()
}
