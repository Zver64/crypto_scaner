package httpapi_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/platform/logging"
)

func TestAPIDocsAreDisabledByDefault(t *testing.T) {
	handler := httpapi.New(logging.New(io.Discard, "error"), readinessStub{}, unavailableAnalysis{}, nil, passThroughAuthenticator{})
	for _, path := range []string{"/docs/", "/openapi.yaml"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", path, response.Code)
		}
	}
}

func TestAPIDocsExposeEmbeddedContractAndSwaggerUIWhenEnabled(t *testing.T) {
	handler := httpapi.NewWithOptions(logging.New(io.Discard, "error"), readinessStub{}, unavailableAnalysis{}, nil, passThroughAuthenticator{}, httpapi.Options{APIDocsEnabled: true})

	contract := httptest.NewRecorder()
	handler.ServeHTTP(contract, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if contract.Code != http.StatusOK || !strings.Contains(contract.Body.String(), "openapi: 3.1.0") {
		t.Fatalf("GET /openapi.yaml status=%d body=%q", contract.Code, contract.Body.String())
	}
	if !strings.HasPrefix(contract.Header().Get("Content-Type"), "application/yaml") {
		t.Errorf("Content-Type = %q, want application/yaml", contract.Header().Get("Content-Type"))
	}

	docs := httptest.NewRecorder()
	handler.ServeHTTP(docs, httptest.NewRequest(http.MethodGet, "/docs/", nil))
	if docs.Code != http.StatusOK || !strings.Contains(docs.Body.String(), "/openapi.yaml") {
		t.Fatalf("GET /docs/ status=%d body=%q", docs.Code, docs.Body.String())
	}
}
