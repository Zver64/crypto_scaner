package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"crypto-scanner/internal/analysis"
)

// Readiness exposes only the operational checks required by the health route.
type Readiness interface {
	DatabaseReady(context.Context) bool
	MigrationsReady(context.Context) bool
	SuccessfulMarketSyncExists(context.Context) bool
}

// Authenticator protects business endpoints with an authenticated user.
type Authenticator interface {
	Authenticate(http.Handler) http.Handler
}

// Analysis exposes the application use cases served by the HTTP API.
type Analysis interface {
	AnalyzeSymbol(context.Context, analysis.SymbolRequest) (analysis.SymbolResult, error)
	Search(context.Context, analysis.SearchRequest) (analysis.SearchResult, error)
}

// New returns the service HTTP handler with process-wide middleware applied.
func New(logger *slog.Logger, readiness Readiness, service Analysis, authenticator Authenticator) http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /health/live", live)
	router.HandleFunc("GET /health/ready", ready(readiness))
	router.Handle("POST /api/v1/analysis/instruments/{symbol}", authenticator.Authenticate(analyzeSymbol(service)))
	router.Handle("POST /api/v1/analysis/market", authenticator.Authenticate(searchMarket(service)))
	return requestMiddleware(logger, router)
}

func live(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte("{\"status\":\"ok\"}\n"))
}

func ready(readiness Readiness) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		checks := map[string]string{
			"database":    "unavailable",
			"migrations":  "unavailable",
			"market_sync": "unavailable",
		}
		status := http.StatusServiceUnavailable
		state := "not_ready"
		if readiness.DatabaseReady(request.Context()) {
			checks["database"] = "ok"
			if readiness.MigrationsReady(request.Context()) {
				checks["migrations"] = "ok"
				if readiness.SuccessfulMarketSyncExists(request.Context()) {
					checks["market_sync"] = "ok"
					status = http.StatusOK
					state = "ready"
				} else {
					checks["market_sync"] = "missing"
				}
			}
		}

		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(status)
		_ = json.NewEncoder(response).Encode(struct {
			Status string            `json:"status"`
			Checks map[string]string `json:"checks"`
		}{Status: state, Checks: checks})
	}
}
