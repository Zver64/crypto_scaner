package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/market"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/oapi-codegen/nethttp-middleware"
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

const maxAnalysisRequestBody = 1 << 20

type Options struct {
	APIDocsEnabled    bool
	Chart             ChartService
	LiveAuthenticator LiveAuthenticator
	LiveCandles       LiveCandles
}

type api struct {
	readiness Readiness
	analysis  Analysis
	history   CandleHistory
	chart     ChartService
}

var _ StrictServerInterface = (*api)(nil)

// New returns the service HTTP handler with process-wide middleware applied.
func New(logger *slog.Logger, readiness Readiness, service Analysis, history CandleHistory, authenticator Authenticator) http.Handler {
	return NewWithOptions(logger, readiness, service, history, authenticator, Options{})
}

// NewWithOptions returns the service HTTP handler with optional development-only API documentation.
func NewWithOptions(logger *slog.Logger, readiness Readiness, service Analysis, history CandleHistory, authenticator Authenticator, options Options) http.Handler {
	operations := http.NewServeMux()
	strict := NewStrictHandlerWithOptions(&api{readiness: readiness, analysis: service, history: history, chart: options.Chart}, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  openAPIRequestError,
		ResponseErrorHandlerFunc: openAPIResponseError,
	})
	HandlerWithOptions(strict, StdHTTPServerOptions{BaseRouter: operations, ErrorHandlerFunc: openAPIRequestError})

	validator, err := openAPIValidator()
	if err != nil {
		panic(fmt.Sprintf("load OpenAPI contract: %v", err))
	}

	router := http.NewServeMux()
	router.Handle("/health/", operations)
	protectedOperations := authenticator.Authenticate(defaultJSONContentType(limitAnalysisRequestBody(validator(operations))))
	router.Handle("POST /api/v1/analysis/instruments/{symbol}", protectedOperations)
	router.Handle("POST /api/v1/analysis/market", protectedOperations)
	router.Handle("GET /api/v1/instruments/{symbol}/candles", protectedOperations)
	router.Handle("POST /api/v1/instruments/{symbol}/chart", protectedOperations)
	if options.LiveAuthenticator != nil && options.LiveCandles != nil {
		router.Handle("GET /api/v1/live/candles", newLiveCandleHandler(options.LiveAuthenticator, options.LiveCandles, logger))
	}
	if options.APIDocsEnabled {
		registerDocs(router)
	}
	return requestMiddleware(logger, router)
}

func openAPIValidator() (func(http.Handler) http.Handler, error) {
	spec, err := GetSpec()
	if err != nil {
		return nil, err
	}
	spec.Servers = nil
	return nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
		Options:              openapi3filter.Options{AuthenticationFunc: func(context.Context, *openapi3filter.AuthenticationInput) error { return nil }},
		ErrorHandlerWithOpts: openAPIValidationError,
	}), nil
}

func openAPIValidationError(_ context.Context, _ error, response http.ResponseWriter, request *http.Request, options nethttpmiddleware.ErrorHandlerOpts) {
	message := "Invalid request"
	switch {
	case strings.HasPrefix(request.URL.Path, "/api/v1/analysis/"):
		message = "Invalid analysis argument"
	case isChartDataPath(request.URL.Path):
		message = candleValidationMessage(request, options)
	}
	writeAPIError(response, options.StatusCode, "invalid_argument", message, nil)
}

func isChartDataPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/instruments/") && (strings.HasSuffix(path, "/candles") || strings.HasSuffix(path, "/chart"))
}

func candleValidationMessage(request *http.Request, options nethttpmiddleware.ErrorHandlerOpts) string {
	query := request.URL.Query()
	if !market.CandleInterval(query.Get("interval")).Valid() {
		return "Unsupported candle interval"
	}
	if values, present := query["limit"]; present {
		if len(values) != 1 {
			return "Invalid candle page limit"
		}
		limit, err := strconv.Atoi(values[0])
		if err != nil || limit < 1 || limit > maxCandlePageSize {
			return "Invalid candle page limit"
		}
	}
	if values, present := query["before"]; present {
		if len(values) != 1 {
			return "Invalid candle cursor"
		}
		if _, err := time.Parse(time.RFC3339, values[0]); err != nil {
			return "Invalid candle cursor"
		}
	}
	if options.MatchedRoute != nil && strings.TrimSpace(options.MatchedRoute.PathParams["symbol"]) == "" {
		return "Symbol is required"
	}
	return "Invalid request"
}

func limitAnalysisRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			request.Body = http.MaxBytesReader(response, request.Body, maxAnalysisRequestBody)
		}
		next.ServeHTTP(response, request)
	})
}

func defaultJSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost && request.Header.Get("Content-Type") == "" {
			request.Header.Set("Content-Type", "application/json")
		}
		next.ServeHTTP(response, request)
	})
}

func openAPIRequestError(response http.ResponseWriter, request *http.Request, _ error) {
	message := "Invalid request"
	switch {
	case strings.HasPrefix(request.URL.Path, "/api/v1/analysis/"):
		message = "Invalid analysis argument"
	case isChartDataPath(request.URL.Path):
		message = candleValidationMessage(request, nethttpmiddleware.ErrorHandlerOpts{})
	}
	writeAPIError(response, http.StatusBadRequest, "invalid_argument", message, nil)
}

func openAPIResponseError(response http.ResponseWriter, _ *http.Request, _ error) {
	writeAPIError(response, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
}

func (api *api) GetLiveness(ctx context.Context, _ GetLivenessRequestObject) (GetLivenessResponseObject, error) {
	return GetLiveness200JSONResponse{
		Body:    LivenessResponse{Status: LivenessResponseStatusOk},
		Headers: GetLiveness200ResponseHeaders{XRequestID: RequestIdentifier(ctx)},
	}, nil
}

func (api *api) GetReadiness(ctx context.Context, _ GetReadinessRequestObject) (GetReadinessResponseObject, error) {
	checks := struct {
		Database   ReadinessCheck `json:"database"`
		MarketSync ReadinessCheck `json:"market_sync"`
		Migrations ReadinessCheck `json:"migrations"`
	}{Database: "unavailable", Migrations: "unavailable", MarketSync: "unavailable"}
	status := http.StatusServiceUnavailable
	state := NotReady
	if api.readiness.DatabaseReady(ctx) {
		checks.Database = "ok"
		if api.readiness.MigrationsReady(ctx) {
			checks.Migrations = "ok"
			if api.readiness.SuccessfulMarketSyncExists(ctx) {
				checks.MarketSync = "ok"
				status = http.StatusOK
				state = Ready
			} else {
				checks.MarketSync = "missing"
			}
		}
	}
	body := ReadinessResponse{Status: state}
	body.Checks = checks
	if status == http.StatusOK {
		return GetReadiness200JSONResponse{Body: body, Headers: GetReadiness200ResponseHeaders{XRequestID: RequestIdentifier(ctx)}}, nil
	}
	return GetReadiness503JSONResponse{Body: body, Headers: GetReadiness503ResponseHeaders{XRequestID: RequestIdentifier(ctx)}}, nil
}
