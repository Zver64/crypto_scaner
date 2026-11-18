package httpapi

import (
	"log/slog"
	"net/http"
)

// NewWithAuthentication builds the handler like New with a test
// authentication middleware in place of session verification.
func NewWithAuthentication(logger *slog.Logger, dependencies Dependencies, options Options, authenticate func(http.Handler) http.Handler) (http.Handler, error) {
	if err := dependencies.check(); err != nil {
		return nil, err
	}
	return newHandler(logger, dependencies, options, authenticate), nil
}

// RequireSession exposes the session authentication middleware to tests.
var RequireSession = requireSession
