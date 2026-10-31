// Package opsnotify delivers the process's warning and error log records to
// the administrator, collapsing repeats into one hourly summary.
package opsnotify

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"crypto-scanner/internal/telegramhtml"
)

const (
	queueSize = 256
	// maxLineLength caps each line of a message, so one long line cannot
	// push the message and the other attributes past Telegram's limit.
	maxLineLength = 1000
	// repeatWindow is how long repeats of a delivered record are only counted.
	repeatWindow = time.Hour
	flushPeriod  = time.Minute
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

// message is one notification and the level of the record it reports.
type message struct {
	level slog.Level
	text  string
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
	coins  telegramhtml.Coins
	// logger must not forward to queue, or a failed delivery would be queued.
	logger *slog.Logger
	groups map[string]*group
	// unsent holds the messages whose delivery Run's cancellation aborted;
	// Flush sends them.
	unsent []message
}

func New(queue *Queue, sender Sender, coins telegramhtml.Coins, logger *slog.Logger) (*Notifier, error) {
	if queue == nil || sender == nil || logger == nil {
		return nil, fmt.Errorf("invalid operations notifier")
	}
	return &Notifier{queue: queue, sender: sender, coins: coins, logger: logger.With("module", "ops_notify"), groups: map[string]*group{}}, nil
}

func (n *Notifier) Run(ctx context.Context) error {
	ticker := time.NewTicker(flushPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case record := <-n.queue.records:
			if first, ok := n.receive(record); ok {
				n.send(ctx, first)
			}
		case now := <-ticker.C:
			for _, summary := range n.summaries(now, repeatWindow) {
				n.send(ctx, summary)
			}
			n.sendDropped(ctx)
		}
	}
}

// Flush sends, for the shutdown of the process, the messages whose delivery
// Run's cancellation aborted, the records still queued, and every pending
// summary, errors before warnings. Call it only after Run returned.
func (n *Notifier) Flush(ctx context.Context) {
	messages := n.unsent
	n.unsent = nil
	for drained := false; !drained; {
		select {
		case record := <-n.queue.records:
			if first, ok := n.receive(record); ok {
				messages = append(messages, first)
			}
		default:
			drained = true
		}
	}
	messages = append(messages, n.summaries(time.Now(), 0)...)
	slices.SortStableFunc(messages, func(a, b message) int { return cmp.Compare(b.level, a.level) })
	for _, item := range messages {
		n.send(ctx, item)
	}
	n.sendDropped(ctx)
}

// receive groups record and returns its message when it is the first of its
// group; repeats are only counted.
func (n *Notifier) receive(record slog.Record) (message, bool) {
	key := record.Level.String() + "\x00" + module(record) + "\x00" + record.Message
	if existing, ok := n.groups[key]; ok {
		existing.repeats++
		existing.latest = record
		return message{}, false
	}
	n.groups[key] = &group{since: time.Now(), latest: record}
	return message{level: record.Level, text: format(n.coins, record, 0)}, true
}

// summaries returns the summaries of the repeats of groups older than window
// and forgets the groups without repeats.
func (n *Notifier) summaries(now time.Time, window time.Duration) []message {
	var result []message
	for key, existing := range n.groups {
		if now.Sub(existing.since) < window {
			continue
		}
		if existing.repeats == 0 {
			delete(n.groups, key)
			continue
		}
		result = append(result, message{level: existing.latest.Level, text: format(n.coins, existing.latest, existing.repeats)})
		existing.since, existing.repeats = now, 0
	}
	return result
}

func (n *Notifier) sendDropped(ctx context.Context) {
	if dropped := n.queue.dropped.Swap(0); dropped > 0 {
		n.send(ctx, message{level: slog.LevelWarn, text: telegramhtml.Title(fmt.Sprintf("⚠️ %d log records were dropped because the notification queue was full", dropped))})
	}
}

// send delivers item; a delivery that ctx's cancellation aborts is kept for
// Flush, so stopping Run never loses the message it was sending.
func (n *Notifier) send(ctx context.Context, item message) {
	err := n.sender.SendAdministratorMessage(ctx, item.text)
	switch {
	case err == nil:
	case ctx.Err() != nil:
		n.unsent = append(n.unsent, item)
	default:
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

// format writes record as Telegram HTML: a bold title with the level and
// module, the message, and a line per attribute, its value monospaced or, for
// a symbol, linked to the coin. The message and values are joined into one
// line each and capped, so a message too long for Telegram ends at a whole
// line and keeps the message and the first attributes.
func format(coins telegramhtml.Coins, record slog.Record, repeats int) string {
	icon := "⚠️"
	if record.Level >= slog.LevelError {
		icon = "🛑"
	}
	title := icon + " " + record.Level.String()
	if name := module(record); name != "" {
		title += " · " + name
	}
	lines := []string{telegramhtml.Title(title), line(record.Message)}
	if repeats > 0 {
		lines = append(lines, fmt.Sprintf("Repeated %d times in the last hour; latest:", repeats))
	}
	record.Attrs(func(attr slog.Attr) bool {
		switch attr.Key {
		case "module": // in the title
		case "symbol":
			lines = append(lines, "symbol: "+coins.Symbol(attr.Value.String()))
		default:
			lines = append(lines, line(attr.Key)+": <code>"+line(attr.Value.String())+"</code>")
		}
		return true
	})
	return telegramhtml.Truncate(strings.Join(lines, "\n"), "\n…")
}

// line escapes text, joins its lines, and caps it at maxLineLength bytes,
// cutting neither a character nor an HTML entity.
func line(text string) string {
	escaped := telegramhtml.OneLine(text)
	if len(escaped) <= maxLineLength {
		return escaped
	}
	cut := escaped[:maxLineLength-len("…")]
	if entity := strings.LastIndexByte(cut, '&'); entity >= 0 && !strings.Contains(cut[entity:], ";") {
		cut = cut[:entity]
	}
	for !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut + "…"
}
