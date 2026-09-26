package httpapi

import (
	"log/slog"
	"net/http"
)

// NewWithAuthentication builds the handler with a test authentication
// middleware in place of Telegram init-data verification.
func NewWithAuthentication(logger *slog.Logger, dependencies Dependencies, options Options, authenticate func(http.Handler) http.Handler) http.Handler {
	return newHandler(logger, dependencies, options, authenticate)
}

// RequireTelegramUser exposes the Telegram authentication middleware to tests.
var RequireTelegramUser = requireTelegramUser
