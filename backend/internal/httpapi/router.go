package httpapi

import (
	"cmp"
	"context"
	"errors"
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
	"crypto-scanner/internal/markettable"

	"github.com/getkin/kin-openapi/openapi3"
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
}

// MarketTables runs market and favorites analyses and presents them as tables.
type MarketTables interface {
	Search(context.Context, analysis.SearchRequest) (markettable.Result, error)
	Favorites(context.Context, []favorites.Favorite, analysis.SearchRequest) (markettable.Result, error)
}

type Favorites interface {
	List(context.Context, int64) ([]favorites.Favorite, error)
	Add(context.Context, int64, string) (favorites.Favorite, error)
	Remove(context.Context, int64, string, bool) (int, error)
}

type PriceAlerts interface {
	List(context.Context, int64, string) ([]alerts.Alert, error)
	Create(context.Context, int64, string, string) (alerts.Alert, error)
	Update(context.Context, int64, int64, string) (alerts.Alert, error)
	Delete(context.Context, int64, int64) error
}

const maxRequestBody = 1 << 20

// Dependencies are the use cases served by the API. All are required.
type Dependencies struct {
	Readiness    Readiness
	Analysis     Analysis
	MarketTables MarketTables
	History      CandleHistory
	GridLimits   GridLimits
	// Sessions issues session tokens and authenticates HTTP and WebSocket
	// requests by them.
	Sessions Sessions
	// APITokens serves the administrator's API tokens.
	APITokens   APITokens
	Chart       ChartService
	LiveCandles LiveCandles
	Favorites   Favorites
	Alerts      PriceAlerts
	// ScannerIndicators and IndicatorTypes serve the administrator settings.
	ScannerIndicators ScannerIndicators
	IndicatorTypes    IndicatorTypes
	// Users serves the administrator's user management.
	Users Users
	// Strategies serves the administrator's strategies.
	Strategies Strategies
	// HistoryLoads loads deeper candle history for backtests.
	HistoryLoads HistoryLoads
}

type Options struct {
	APIDocsEnabled bool
}

type api struct {
	logger     *slog.Logger
	readiness  Readiness
	analysis   Analysis
	tables     MarketTables
	history    CandleHistory
	gridLimits GridLimits
	favorites  Favorites
	alerts     PriceAlerts
	chart      ChartService
	sessions   Sessions
	apiTokens  APITokens

	scannerIndicators ScannerIndicators
	indicatorTypes    IndicatorTypes
	users             Users
	strategies        Strategies
	historyLoads      HistoryLoads
}

var _ StrictServerInterface = (*api)(nil)

// protectedRoutes are the authenticated OpenAPI operations. Method-specific
// patterns let the router reject unsupported methods before authentication.
var protectedRoutes = []string{
	"POST /api/v1/analysis/instruments/{symbol}",
	"POST /api/v1/analysis/market",
	"GET /api/v1/instruments/{symbol}/candles",
	"GET /api/v1/instruments/{symbol}/grid-limits",
	"GET /api/v1/chart/indicators",
	"GET /api/v1/instruments",
	"GET /api/v1/favorites",
	"PUT /api/v1/favorites/{symbol}",
	"DELETE /api/v1/favorites/{symbol}",
	"POST /api/v1/favorites/analysis",
	"GET /api/v1/instruments/{symbol}/alerts",
	"POST /api/v1/instruments/{symbol}/alerts",
	"PATCH /api/v1/alerts/{alert_id}",
	"DELETE /api/v1/alerts/{alert_id}",
	"GET /api/v1/me",
	"DELETE /api/v1/auth/session",
}

// administratorRoutes are the operations only the scanner administrator may
// call. They are authenticated like protectedRoutes.
var administratorRoutes = []string{
	"GET /api/v1/admin/indicator-types",
	"GET /api/v1/admin/scanner-indicators",
	"POST /api/v1/admin/scanner-indicators",
	"DELETE /api/v1/admin/scanner-indicators",
	"PATCH /api/v1/admin/scanner-indicators/{indicator_id}",
	"DELETE /api/v1/admin/scanner-indicators/{indicator_id}",
	"POST /api/v1/admin/scanner-indicator-batches",
	"PUT /api/v1/admin/scanner-indicator-order",
	"GET /api/v1/admin/users",
	"PATCH /api/v1/admin/users/{telegram_id}",
	"DELETE /api/v1/admin/users/{telegram_id}",
	"GET /api/v1/admin/strategy-variables",
	"GET /api/v1/admin/strategy-symbols",
	"POST /api/v1/admin/strategy-validations",
	"GET /api/v1/admin/strategies",
	"POST /api/v1/admin/strategies",
	"PUT /api/v1/admin/strategies/{strategy_id}",
	"PATCH /api/v1/admin/strategies/{strategy_id}",
	"DELETE /api/v1/admin/strategies/{strategy_id}",
	"GET /api/v1/admin/strategies/{strategy_id}/backtest",
	"POST /api/v1/admin/strategy-backtests",
	"POST /api/v1/admin/candle-history-loads",
	"GET /api/v1/admin/candle-history-loads",
	"GET /api/v1/admin/api-tokens",
	"POST /api/v1/admin/api-tokens",
	"DELETE /api/v1/admin/api-tokens/{token_id}",
}

// New returns the service HTTP handler with process-wide middleware applied.
func New(logger *slog.Logger, dependencies Dependencies, options Options) http.Handler {
	return newHandler(logger, dependencies, options, requireSession(dependencies.Sessions, logger))
}

func newHandler(logger *slog.Logger, dependencies Dependencies, options Options, authenticate func(http.Handler) http.Handler) http.Handler {
	operations := http.NewServeMux()
	handlers := &api{logger: logger, readiness: dependencies.Readiness, analysis: dependencies.Analysis, tables: dependencies.MarketTables, history: dependencies.History, gridLimits: dependencies.GridLimits, favorites: dependencies.Favorites, alerts: dependencies.Alerts, chart: dependencies.Chart, sessions: dependencies.Sessions, apiTokens: dependencies.APITokens,
		scannerIndicators: dependencies.ScannerIndicators, indicatorTypes: dependencies.IndicatorTypes, users: dependencies.Users, strategies: dependencies.Strategies, historyLoads: dependencies.HistoryLoads}
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
	router.Handle("POST /api/v1/auth/session", requireInitData(limitRequestBody(validator(operations))))
	protectedOperations := authenticate(defaultJSONContentType(limitRequestBody(validator(operations))))
	for _, route := range protectedRoutes {
		router.Handle(route, protectedOperations)
	}
	administratorOperations := authenticate(requireAdministrator(defaultJSONContentType(limitRequestBody(validator(operations)))))
	for _, route := range administratorRoutes {
		router.Handle(route, administratorOperations)
	}
	router.Handle("GET /api/v1/live/candles", newLiveCandleHandler(dependencies.Sessions, dependencies.LiveCandles, dependencies.Chart, logger))
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

func openAPIValidationError(_ context.Context, err error, response http.ResponseWriter, request *http.Request, options nethttpmiddleware.ErrorHandlerOpts) {
	writeAPIError(response, options.StatusCode, "invalid_argument", validationMessage(request, options, err), nil)
}

func validationMessage(request *http.Request, options nethttpmiddleware.ErrorHandlerOpts, err error) string {
	switch {
	case strings.HasPrefix(request.URL.Path, "/api/v1/analysis/"):
		return "Invalid analysis argument"
	case isCandleHistoryPath(request.URL.Path):
		return candleValidationMessage(request, options)
	default:
		return requestErrorMessage(err)
	}
}

// requestErrorMessage names the parameter or body field a request fails
// validation on and why, without the schema and value dumps of kin-openapi
// errors.
func requestErrorMessage(err error) string {
	var failure *openapi3filter.RequestError
	if !errors.As(err, &failure) {
		return "Invalid request"
	}
	reason, field := failure.Reason, ""
	var schema *openapi3.SchemaError
	var parsing *openapi3filter.ParseError
	if errors.As(failure.Err, &schema) {
		reason = cmp.Or(schema.Reason, "does not match the schema")
		field = strings.Join(schema.JSONPointer(), "/")
	} else if errors.As(failure.Err, &parsing) {
		// Error() includes the complete input value and nested causes.
		// The typed reason explains the failure without echoing that input.
		reason = cmp.Or(parsing.Reason, reason, "invalid format")
	} else if failure.Err != nil {
		reason = cmp.Or(reason, failure.Err.Error())
	}
	reason = cmp.Or(conciseReason(reason), "invalid value")
	switch {
	case failure.Parameter != nil:
		return fmt.Sprintf("Invalid %s parameter %q: %s", failure.Parameter.In, failure.Parameter.Name, reason)
	case field != "":
		return fmt.Sprintf("Invalid request body field %q: %s", field, reason)
	case failure.RequestBody != nil:
		return "Invalid request body: " + reason
	}
	return "Invalid request: " + reason
}

// maxReasonLength bounds the reason of a validation message.
const maxReasonLength = 200

// conciseReason keeps the findings of a JSON Schema 2020 validation error
// (OpenAPI 3.1 parameters), whose first line only names the schema, on one
// line.
func conciseReason(reason string) string {
	if header, findings, found := strings.Cut(reason, "\n"); found && strings.HasPrefix(header, "jsonschema validation failed") {
		reason = findings
	}
	var parts []string
	for line := range strings.Lines(reason) {
		line = strings.TrimPrefix(strings.TrimSpace(line), "- ")
		if line = strings.TrimPrefix(line, "at '': "); line != "" {
			parts = append(parts, line)
		}
	}
	reason = strings.Join(parts, "; ")
	if len(reason) > maxReasonLength {
		reason = strings.ToValidUTF8(reason[:maxReasonLength], "") + "…"
	}
	return reason
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

// limitRequestBody bounds the body of every request, whatever its method.
func limitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(response, request.Body, maxRequestBody)
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

func openAPIRequestError(response http.ResponseWriter, request *http.Request, err error) {
	writeAPIError(response, http.StatusBadRequest, "invalid_argument", validationMessage(request, nethttpmiddleware.ErrorHandlerOpts{}, err), nil)
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
