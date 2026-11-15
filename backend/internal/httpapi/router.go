package httpapi

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
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

// MarketTables runs market, COIN-M, and favorites analyses and presents them
// as tables.
type MarketTables interface {
	Search(context.Context, analysis.SearchRequest) (markettable.Result, error)
	CoinM(context.Context, analysis.SearchRequest) (markettable.Result, error)
	CoinMSymbols(context.Context) ([]string, error)
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

	spec, err := GetSpec()
	if err != nil {
		panic(fmt.Sprintf("load OpenAPI contract: %v", err))
	}
	validator := openAPIValidator(spec)

	router := http.NewServeMux()
	router.Handle("/health/", operations)
	router.Handle("POST /api/v1/auth/session", requireInitData(limitRequestBody(validator(operations))))
	if err := registerSecuredOperations(router, spec, authenticate, defaultJSONContentType(limitRequestBody(validator(operations)))); err != nil {
		panic(fmt.Sprintf("route OpenAPI contract: %v", err))
	}
	router.Handle("GET /api/v1/live/candles", newLiveCandleHandler(dependencies.Sessions, dependencies.LiveCandles, dependencies.Chart, logger))
	if options.APIDocsEnabled {
		registerDocs(router)
	}
	return requestMiddleware(logger, router)
}

func openAPIValidator(spec *openapi3.T) func(http.Handler) http.Handler {
	spec.Servers = nil
	return nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
		Options:              openapi3filter.Options{AuthenticationFunc: func(context.Context, *openapi3filter.AuthenticationInput) error { return nil }},
		ErrorHandlerWithOpts: openAPIValidationError,
	})
}

// Values of the x-access extension of operations secured by sessionToken.
const (
	accessUser          = "user"
	accessAdministrator = "administrator"
)

// registerSecuredOperations registers every operation the contract secures
// with sessionToken under its method-specific pattern, so unsupported methods
// and unknown paths are rejected before authentication. Each runs behind
// authentication, then the administrator check its x-access asks for, then
// the API token refusal its x-session-only asks for. A missing or unknown
// access value is an error, never an open route.
func registerSecuredOperations(router *http.ServeMux, spec *openapi3.T, authenticate func(http.Handler) http.Handler, next http.Handler) error {
	for path, item := range spec.Paths.Map() {
		for method, operation := range item.Operations() {
			security := spec.Security
			if operation.Security != nil {
				security = *operation.Security
			}
			if !slices.ContainsFunc(security, func(requirement openapi3.SecurityRequirement) bool { _, ok := requirement["sessionToken"]; return ok }) {
				if _, ok := operation.Extensions["x-access"]; ok {
					return fmt.Errorf("%s %s declares x-access without sessionToken security", method, path)
				}
				if _, ok := operation.Extensions["x-session-only"]; ok {
					return fmt.Errorf("%s %s declares x-session-only without sessionToken security", method, path)
				}
				continue
			}
			handler := next
			if value, ok := operation.Extensions["x-session-only"]; ok {
				message, _ := value.(string)
				if message == "" {
					return fmt.Errorf("%s %s: x-session-only must be the refusal message", method, path)
				}
				handler = refuseAPITokens(message)(handler)
			}
			switch access, _ := operation.Extensions["x-access"].(string); access {
			case accessUser:
			case accessAdministrator:
				handler = requireAdministrator(handler)
			default:
				return fmt.Errorf("%s %s: x-access is %q, want %q or %q", method, path, access, accessUser, accessAdministrator)
			}
			router.Handle(method+" "+path, authenticate(handler))
		}
	}
	return nil
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
