package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"crypto-scanner/internal/analysis"
)

// apiError is an error body with its HTTP status. The helpers below convert it
// into generated response components that declare X-Request-ID; components
// without response headers use body directly.
type apiError struct {
	status int
	body   ErrorResponse
}

func newAPIError(ctx context.Context, status int, code, message string, details any) apiError {
	return apiError{status: status, body: ErrorResponse{Error: APIError{Code: APIErrorCode(code), Message: message, Details: details}, RequestId: RequestIdentifier(ctx)}}
}

func (e apiError) badRequest() BadRequestJSONResponse {
	return BadRequestJSONResponse{Body: e.body, Headers: BadRequestResponseHeaders{XRequestID: e.body.RequestId}}
}

func (e apiError) symbolNotFound() SymbolNotFoundJSONResponse {
	return SymbolNotFoundJSONResponse{Body: e.body, Headers: SymbolNotFoundResponseHeaders{XRequestID: e.body.RequestId}}
}

func (e apiError) insufficientData() InsufficientDataJSONResponse {
	return InsufficientDataJSONResponse{Body: e.body, Headers: InsufficientDataResponseHeaders{XRequestID: e.body.RequestId}}
}

func (e apiError) unprocessable() UnprocessableAnalysisJSONResponse {
	return UnprocessableAnalysisJSONResponse{Body: e.body, Headers: UnprocessableAnalysisResponseHeaders{XRequestID: e.body.RequestId}}
}

func (e apiError) unavailable() AnalysisUnavailableJSONResponse {
	return AnalysisUnavailableJSONResponse{Body: e.body, Headers: AnalysisUnavailableResponseHeaders{XRequestID: e.body.RequestId}}
}

func (e apiError) internal() InternalErrorJSONResponse {
	return InternalErrorJSONResponse{Body: e.body, Headers: InternalErrorResponseHeaders{XRequestID: e.body.RequestId}}
}

func invalidArgument(ctx context.Context, message string) apiError {
	return newAPIError(ctx, http.StatusBadRequest, "invalid_argument", message, nil)
}

func symbolNotFound(ctx context.Context) apiError {
	return newAPIError(ctx, http.StatusNotFound, "symbol_not_found", "Symbol is unknown or inactive", nil)
}

// internalError logs the cause, which never reaches the client, and returns
// the generic 500 body.
func (api *api) internalError(ctx context.Context, operation string, err error) InternalErrorJSONResponse {
	api.logger.ErrorContext(ctx, "HTTP operation failed",
		"module", "httpapi", "operation", operation, "request_id", RequestIdentifier(ctx), "error", err)
	return newAPIError(ctx, http.StatusInternalServerError, "internal_error", "Internal server error", nil).internal()
}

// analysisError maps analysis use-case errors; ok is false for unexpected
// errors, which callers report through internalError.
func analysisError(ctx context.Context, err error, symbol string) (apiError, bool) {
	var insufficient *analysis.InsufficientHistoryError
	var unresolved *analysis.UnresolvedError
	switch {
	case errors.Is(err, analysis.ErrInvalidArgument):
		return invalidArgument(ctx, "Invalid analysis argument"), true
	case errors.Is(err, analysis.ErrSymbolNotFound):
		return symbolNotFound(ctx), true
	case errors.As(err, &insufficient):
		return newAPIError(ctx, http.StatusConflict, "insufficient_data", "Not enough closed candles for the requested period", map[string]any{"symbol": symbol, "criterion": insufficient.Criterion, "required": insufficient.Required, "available": insufficient.Available}), true
	case errors.Is(err, analysis.ErrMarketDataUnavailable):
		return newAPIError(ctx, http.StatusServiceUnavailable, "market_data_unavailable", "Market data is unavailable", nil), true
	case errors.As(err, &unresolved):
		return newAPIError(ctx, http.StatusUnprocessableEntity, unresolved.Code, unresolved.Message, map[string]any{"symbol": symbol}), true
	default:
		return apiError{}, false
	}
}

func writeAPIError(response http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(response, status, ErrorResponse{Error: APIError{Code: APIErrorCode(code), Message: message, Details: details}, RequestId: response.Header().Get("X-Request-ID")})
}

func writeJSON(response http.ResponseWriter, status int, body any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(body)
}
