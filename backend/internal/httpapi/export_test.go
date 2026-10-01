package httpapi

import (
	"log/slog"
	"net/http"
)

// NewWithAuthentication builds the handler with a test authentication
// middleware in place of session verification.
func NewWithAuthentication(logger *slog.Logger, dependencies Dependencies, options Options, authenticate func(http.Handler) http.Handler) http.Handler {
	return newHandler(logger, dependencies, options, authenticate)
}

// RequireSession exposes the session authentication middleware to tests.
var RequireSession = requireSession
