package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"crypto-scanner/internal/alerts"
	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/favorites"
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

// Analysis exposes the application use cases served by the HTTP API.
type Analysis interface {
	AnalyzeSymbol(context.Context, analysis.SymbolRequest) (analysis.SymbolResult, error)
	Search(context.Context, analysis.SearchRequest) (analysis.SearchResult, error)
}

type Favorites interface {
	List(context.Context, int64) ([]favorites.Favorite, error)
	Add(context.Context, int64, string) (favorites.Favorite, error)
	Remove(context.Context, int64, string, bool) (int, error)
	Analyze(context.Context, int64, analysis.SearchRequest) (analysis.SearchResult, error)
}

type PriceAlerts interface {
	List(context.Context, int64, string) ([]alerts.Alert, error)
	Create(context.Context, int64, string, string) (alerts.Alert, error)
	Update(context.Context, int64, int64, string) (alerts.Alert, error)
	Delete(context.Context, int64, int64) error
}

const maxAnalysisRequestBody = 1 << 20

// Dependencies are the use cases served by the API. All are required.
type Dependencies struct {
	Readiness Readiness
	Analysis  Analysis
	History   CandleHistory
	// Authenticator verifies Telegram init data for HTTP and WebSocket requests.
	Authenticator InitDataAuthenticator
	Chart         ChartService
	LiveCandles   LiveCandles
	Favorites     Favorites
	Alerts        PriceAlerts
}

type Options struct {
	APIDocsEnabled bool
}

type api struct {
	logger    *slog.Logger
	readiness Readiness
	analysis  Analysis
	history   CandleHistory
	favorites Favorites
	alerts    PriceAlerts
}

var _ StrictServerInterface = (*api)(nil)

// protectedRoutes are the authenticated OpenAPI operations. Method-specific
// patterns let the router reject unsupported methods before authentication.
var protectedRoutes = []string{
	"POST /api/v1/analysis/instruments/{symbol}",
	"POST /api/v1/analysis/market",
	"GET /api/v1/instruments/{symbol}/candles",
	"GET /api/v1/favorites",
	"PUT /api/v1/favorites/{symbol}",
	"DELETE /api/v1/favorites/{symbol}",
	"POST /api/v1/favorites/analysis",
	"GET /api/v1/instruments/{symbol}/alerts",
	"POST /api/v1/instruments/{symbol}/alerts",
	"PATCH /api/v1/alerts/{alert_id}",
	"DELETE /api/v1/alerts/{alert_id}",
}

// New returns the service HTTP handler with process-wide middleware applied.
func New(logger *slog.Logger, dependencies Dependencies, options Options) http.Handler {
	return newHandler(logger, dependencies, options, requireTelegramUser(dependencies.Authenticator))
}

func newHandler(logger *slog.Logger, dependencies Dependencies, options Options, authenticate func(http.Handler) http.Handler) http.Handler {
	operations := http.NewServeMux()
	handlers := &api{logger: logger, readiness: dependencies.Readiness, analysis: dependencies.Analysis, history: dependencies.History, favorites: dependencies.Favorites, alerts: dependencies.Alerts}
	strict := NewStrictHandlerWithOptions(handlers, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc: openAPIRequestError,
		ResponseErrorHandlerFunc: func(response http.ResponseWriter, request *http.Request, err error) {
			logger.ErrorContext(request.Context(), "HTTP response failed", "module", "httpapi", "request_id", RequestIdentifier(request.Context()), "error", err)
			writeAPIError(response, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
		},
	})
	HandlerWithOptions(strict, StdHTTPServerOptions{BaseRouter: operations, ErrorHandlerFunc: openAPIRequestError})

	validator, err := openAPIValidator()
	if err != nil {
		panic(fmt.Sprintf("load OpenAPI contract: %v", err))
	}

	router := http.NewServeMux()
	router.Handle("/health/", operations)
	protectedOperations := authenticate(defaultJSONContentType(limitAnalysisRequestBody(validator(operations))))
	for _, route := range protectedRoutes {
		router.Handle(route, protectedOperations)
	}
	router.Handle("GET /api/v1/live/candles", newLiveCandleHandler(dependencies.Authenticator, dependencies.LiveCandles, dependencies.Chart, logger))
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
	writeAPIError(response, options.StatusCode, "invalid_argument", validationMessage(request, options), nil)
}

func validationMessage(request *http.Request, options nethttpmiddleware.ErrorHandlerOpts) string {
	switch {
	case strings.HasPrefix(request.URL.Path, "/api/v1/analysis/"):
		return "Invalid analysis argument"
	case isCandleHistoryPath(request.URL.Path):
		return candleValidationMessage(request, options)
	default:
		return "Invalid request"
	}
}

func isCandleHistoryPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/instruments/") && strings.HasSuffix(path, "/candles")
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
	writeAPIError(response, http.StatusBadRequest, "invalid_argument", validationMessage(request, nethttpmiddleware.ErrorHandlerOpts{}), nil)
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
