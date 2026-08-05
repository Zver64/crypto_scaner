package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/platform/logging"
)

func TestReadinessReportsMissingSuccessfulMarketSync(t *testing.T) {
	handler := newTestHTTPHandler(
		logging.New(io.Discard, "error"),
		readinessStub{database: true, migrations: true},
	)
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode readiness response: %v", err)
	}

	if response.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", response.Code)
	}
	if body.Status != "not_ready" || body.Checks["database"] != "ok" || body.Checks["migrations"] != "ok" || body.Checks["market_sync"] != "missing" {
		t.Fatalf("readiness body = %#v, want missing successful market sync", body)
	}
}

func TestReadinessSucceedsWithDatabaseMigrationsAndSuccessfulMarketSync(t *testing.T) {
	handler := newTestHTTPHandler(
		logging.New(io.Discard, "error"),
		readinessStub{database: true, migrations: true, marketSync: true},
	)
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
}

func TestReadinessRejectsUnavailableDatabaseOrMigrations(t *testing.T) {
	tests := []struct {
		name       string
		readiness  readinessStub
		wantChecks map[string]string
	}{
		{
			name:       "database unavailable",
			readiness:  readinessStub{},
			wantChecks: map[string]string{"database": "unavailable", "migrations": "unavailable", "market_sync": "unavailable"},
		},
		{
			name:       "migrations unavailable",
			readiness:  readinessStub{database: true},
			wantChecks: map[string]string{"database": "ok", "migrations": "unavailable", "market_sync": "unavailable"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := newTestHTTPHandler(logging.New(io.Discard, "error"), test.readiness)
			request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			var body struct {
				Checks map[string]string `json:"checks"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode readiness response: %v", err)
			}
			if response.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", response.Code)
			}
			for name, want := range test.wantChecks {
				if body.Checks[name] != want {
					t.Errorf("check %s = %q, want %q", name, body.Checks[name], want)
				}
			}
		})
	}
}

type readinessStub struct {
	database   bool
	migrations bool
	marketSync bool
}

func (stub readinessStub) DatabaseReady(context.Context) bool              { return stub.database }
func (stub readinessStub) MigrationsReady(context.Context) bool            { return stub.migrations }
func (stub readinessStub) SuccessfulMarketSyncExists(context.Context) bool { return stub.marketSync }

func TestLivenessResponseCarriesARequestIDCorrelatedWithTheRequestLog(t *testing.T) {
	var logOutput bytes.Buffer
	server := httptest.NewServer(newTestHTTPHandler(logging.New(&logOutput, "info"), readinessStub{}))
	t.Cleanup(server.Close)

	request, err := http.NewRequest(http.MethodGet, server.URL+"/health/live", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.Header.Set("X-Request-ID", "client-request-123")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("GET /health/live error = %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read liveness body: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", response.StatusCode)
	}
	if response.Header.Get("X-Request-ID") != "client-request-123" {
		t.Errorf("X-Request-ID = %q, want %q", response.Header.Get("X-Request-ID"), "client-request-123")
	}
	if response.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", response.Header.Get("Content-Type"))
	}
	if strings.TrimSpace(string(body)) != `{"status":"ok"}` {
		t.Errorf("body = %q, want liveness JSON", body)
	}

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logOutput.Bytes()), &entry); err != nil {
		t.Fatalf("request log is not one JSON object: %v; output = %q", err, logOutput.String())
	}
	if entry["request_id"] != "client-request-123" || entry["module"] != "httpapi" || entry["operation"] != "request" || entry["method"] != "GET" || entry["path"] != "/health/live" || entry["status"] != float64(200) || entry["outcome"] != "success" {
		t.Fatalf("request log lacks correlation fields: %#v", entry)
	}
	if _, ok := entry["duration"]; !ok {
		t.Fatalf("request log lacks duration: %#v", entry)
	}
}

func TestEveryResponseGetsAGeneratedRequestIDWhenTheIncomingValueIsUnsafe(t *testing.T) {
	server := httptest.NewServer(newTestHTTPHandler(logging.New(io.Discard, "error"), readinessStub{}))
	t.Cleanup(server.Close)

	request, err := http.NewRequest(http.MethodGet, server.URL+"/does-not-exist", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.Header.Set("X-Request-ID", "unsafe value")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("GET unknown route error = %v", err)
	}
	defer response.Body.Close()

	requestID := response.Header.Get("X-Request-ID")
	if response.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", response.StatusCode)
	}
	if requestID == "" || requestID == "unsafe value" || strings.ContainsAny(requestID, " \r\n\t") {
		t.Errorf("generated X-Request-ID is unsafe: %q", requestID)
	}
}

func TestRouterDoesNotExposeTelegramBotEndpoints(t *testing.T) {
	handler := newTestHTTPHandler(logging.New(io.Discard, "error"), readinessStub{})

	for _, path := range []string{"/telegram/webhook", "/telegram/analysis"} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"update_id":1}`))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("POST %s status = %d, want 404", path, response.Code)
		}
	}
}

func newTestHTTPHandler(logger *slog.Logger, readiness httpapi.Readiness) http.Handler {
	return httpapi.New(logger, readiness, unavailableAnalysis{}, passThroughAuthenticator{})
}

type unavailableAnalysis struct{}

func (unavailableAnalysis) AnalyzeSymbol(context.Context, analysis.SymbolRequest) (analysis.SymbolResult, error) {
	return analysis.SymbolResult{}, analysis.ErrMarketDataUnavailable
}
func (unavailableAnalysis) Search(context.Context, analysis.SearchRequest) (analysis.SearchResult, error) {
	return analysis.SearchResult{}, analysis.ErrMarketDataUnavailable
}

type passThroughAuthenticator struct{}

func (passThroughAuthenticator) Authenticate(next http.Handler) http.Handler { return next }
