package logging

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

const redacted = "[REDACTED]"

// New constructs the process logger. Values supplied as secrets are removed
// from messages and attributes in addition to key-based redaction.
func New(output io.Writer, configuredLevel string, secrets ...string) *slog.Logger {
	jsonHandler := slog.NewJSONHandler(output, &slog.HandlerOptions{Level: parseLevel(configuredLevel)})
	return slog.New(&safeHandler{next: jsonHandler, secrets: nonEmpty(secrets)})
}

type safeHandler struct {
	next    slog.Handler
	secrets []string
}

func (h *safeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *safeHandler) Handle(ctx context.Context, record slog.Record) error {
	safeRecord := slog.NewRecord(record.Time, record.Level, h.clean(record.Message), record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		safeRecord.AddAttrs(h.cleanAttr(attr))
		return true
	})
	return h.next.Handle(ctx, safeRecord)
}

func (h *safeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	safe := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		safe = append(safe, h.cleanAttr(attr))
	}
	return &safeHandler{next: h.next.WithAttrs(safe), secrets: h.secrets}
}

func (h *safeHandler) WithGroup(name string) slog.Handler {
	return &safeHandler{next: h.next.WithGroup(name), secrets: h.secrets}
}

func (h *safeHandler) cleanAttr(attr slog.Attr) slog.Attr {
	if sensitiveKey(attr.Key) {
		return slog.String(attr.Key, redacted)
	}

	value := attr.Value.Resolve()
	switch value.Kind() {
	case slog.KindString:
		attr.Value = slog.StringValue(h.clean(value.String()))
	case slog.KindAny:
		// JSON handlers recursively serialize arbitrary values, bypassing the
		// attribute-level sanitizer. Treat opaque values as sensitive because
		// their nested shape and String methods are not under logger control.
		attr.Value = slog.StringValue(redacted)
	case slog.KindGroup:
		group := value.Group()
		cleaned := make([]slog.Attr, 0, len(group))
		for _, child := range group {
			cleaned = append(cleaned, h.cleanAttr(child))
		}
		attr.Value = slog.GroupValue(cleaned...)
	}
	return attr
}

func (h *safeHandler) clean(value string) string {
	for _, secret := range h.secrets {
		value = strings.ReplaceAll(value, secret, redacted)
	}
	return value
}

func parseLevel(value string) slog.Level {
	switch value {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func sensitiveKey(key string) bool {
	normalized := strings.Map(func(character rune) rune {
		switch {
		case character >= 'a' && character <= 'z':
			return character
		case character >= 'A' && character <= 'Z':
			return character + ('a' - 'A')
		case character >= '0' && character <= '9':
			return character
		default:
			return -1
		}
	}, key)
	return strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "authorization") ||
		strings.Contains(normalized, "databaseurl") ||
		strings.Contains(normalized, "initdata") ||
		normalized == "update" ||
		strings.Contains(normalized, "telegramupdate")
}

func nonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
