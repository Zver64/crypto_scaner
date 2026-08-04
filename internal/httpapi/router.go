package httpapi

import (
	"log/slog"
	"net/http"
)

// New returns the service HTTP handler with process-wide middleware applied.
func New(logger *slog.Logger) http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /health/live", live)
	return requestMiddleware(logger, router)
}

func live(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte("{\"status\":\"ok\"}\n"))
}
