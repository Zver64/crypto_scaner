package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/platform/logging"
)

func TestLivenessResponseCarriesARequestIDCorrelatedWithTheRequestLog(t *testing.T) {
	var logOutput bytes.Buffer
	server := httptest.NewServer(httpapi.New(logging.New(&logOutput, "info")))
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
	server := httptest.NewServer(httpapi.New(logging.New(io.Discard, "error")))
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
