package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"

	"crypto-scanner/internal/analysis"
)

func marketSearchRequest(request MarketAnalysisRequest) analysis.SearchRequest {
	result := analysis.SearchRequest{Criteria: criterionConfigs(request.Criteria)}
	if request.Limit != nil {
		result.Limit = *request.Limit
	}
	if request.Sort != nil {
		result.Sort = &analysis.SearchSort{Field: string(request.Sort.Field), Direction: string(request.Sort.Direction)}
	}
	return result
}

func criterionConfigs(criteria []CriterionRequest) []analysis.CriterionConfig {
	configs := make([]analysis.CriterionConfig, len(criteria))
	for i, criterion := range criteria {
		configs[i] = analysis.CriterionConfig{Key: criterion.Key, Name: criterion.Name, Label: criterion.Label, Parameters: criterion.Parameters}
	}
	return configs
}

func (api *api) AnalyzeInstrument(ctx context.Context, request AnalyzeInstrumentRequestObject) (AnalyzeInstrumentResponseObject, error) {
	body := InstrumentAnalysisRequest{}
	if request.Body != nil {
		body = *request.Body
	}
	result, err := api.analysis.AnalyzeSymbol(ctx, analysis.SymbolRequest{Symbol: request.Symbol, Criteria: criterionConfigs(body.Criteria)})
	if err != nil {
		return api.analyzeInstrumentError(ctx, err, request.Symbol), nil
	}
	return AnalyzeInstrument200JSONResponse{
		Body: InstrumentAnalysisResponse{
			Symbol: result.Symbol, Matched: result.Matched,
			Evaluations: responseEvaluations(result.Evaluations), Warnings: responseWarnings(result.Warnings),
		},
		Headers: AnalyzeInstrument200ResponseHeaders{XRequestID: RequestIdentifier(ctx)},
	}, nil
}

func (api *api) AnalyzeMarket(ctx context.Context, request AnalyzeMarketRequestObject) (AnalyzeMarketResponseObject, error) {
	body := MarketAnalysisRequest{}
	if request.Body != nil {
		body = *request.Body
	}
	result, err := api.analysis.Search(ctx, marketSearchRequest(body))
	if err != nil {
		return api.analyzeMarketError(ctx, err), nil
	}
	items := make([]MarketAnalysisItem, len(result.Items))
	for i, item := range result.Items {
		items[i] = MarketAnalysisItem{Symbol: item.Symbol, Matched: item.Matched, Evaluations: responseMarketScanEvaluations(item.Evaluations), PriceHistory: item.PriceHistory}
	}
	unresolved := make([]UnresolvedInstrument, len(result.Unresolved))
	for i, item := range result.Unresolved {
		unresolved[i] = UnresolvedInstrument{Symbol: item.Symbol, Code: UnresolvedInstrumentCode(item.Code), Message: item.Message}
	}
	return AnalyzeMarket200JSONResponse{
		Body: MarketAnalysisResponse{
			PriceHistoryWindow: PriceHistoryWindow{From: result.PriceHistoryWindow.From, To: result.PriceHistoryWindow.To},
			MatchedCount:       result.MatchedCount, AnalyzedCount: result.AnalyzedCount, InsufficientDataCount: result.InsufficientDataCount,
			Items: items, Unresolved: unresolved, Warnings: responseWarnings(result.Warnings),
		},
		Headers: AnalyzeMarket200ResponseHeaders{XRequestID: RequestIdentifier(ctx)},
	}, nil
}

func (api *api) analyzeInstrumentError(ctx context.Context, err error, symbol string) AnalyzeInstrumentResponseObject {
	body, status := analysisError(ctx, err, symbol)
	requestID := RequestIdentifier(ctx)
	switch status {
	case http.StatusBadRequest:
		return AnalyzeInstrument400JSONResponse{BadRequestJSONResponse: BadRequestJSONResponse{Body: body, Headers: BadRequestResponseHeaders{XRequestID: requestID}}}
	case http.StatusNotFound:
		return AnalyzeInstrument404JSONResponse{SymbolNotFoundJSONResponse: SymbolNotFoundJSONResponse{Body: body, Headers: SymbolNotFoundResponseHeaders{XRequestID: requestID}}}
	case http.StatusConflict:
		return AnalyzeInstrument409JSONResponse{InsufficientDataJSONResponse: InsufficientDataJSONResponse{Body: body, Headers: InsufficientDataResponseHeaders{XRequestID: requestID}}}
	case http.StatusUnprocessableEntity:
		return AnalyzeInstrument422JSONResponse{UnprocessableAnalysisJSONResponse: UnprocessableAnalysisJSONResponse{Body: body, Headers: UnprocessableAnalysisResponseHeaders{XRequestID: requestID}}}
	case http.StatusServiceUnavailable:
		return AnalyzeInstrument503JSONResponse{AnalysisUnavailableJSONResponse: AnalysisUnavailableJSONResponse{Body: body, Headers: AnalysisUnavailableResponseHeaders{XRequestID: requestID}}}
	default:
		return AnalyzeInstrument500JSONResponse{InternalErrorJSONResponse: InternalErrorJSONResponse{Body: body, Headers: InternalErrorResponseHeaders{XRequestID: requestID}}}
	}
}

func (api *api) analyzeMarketError(ctx context.Context, err error) AnalyzeMarketResponseObject {
	body, status := analysisError(ctx, err, "")
	requestID := RequestIdentifier(ctx)
	switch status {
	case http.StatusBadRequest:
		return AnalyzeMarket400JSONResponse{BadRequestJSONResponse: BadRequestJSONResponse{Body: body, Headers: BadRequestResponseHeaders{XRequestID: requestID}}}
	case http.StatusUnprocessableEntity:
		return AnalyzeMarket422JSONResponse{UnprocessableAnalysisJSONResponse: UnprocessableAnalysisJSONResponse{Body: body, Headers: UnprocessableAnalysisResponseHeaders{XRequestID: requestID}}}
	case http.StatusServiceUnavailable:
		return AnalyzeMarket503JSONResponse{AnalysisUnavailableJSONResponse: AnalysisUnavailableJSONResponse{Body: body, Headers: AnalysisUnavailableResponseHeaders{XRequestID: requestID}}}
	default:
		return AnalyzeMarket500JSONResponse{InternalErrorJSONResponse: InternalErrorJSONResponse{Body: body, Headers: InternalErrorResponseHeaders{XRequestID: requestID}}}
	}
}

func analysisError(ctx context.Context, err error, symbol string) (ErrorResponse, int) {
	var insufficient *analysis.InsufficientHistoryError
	var unresolved *analysis.UnresolvedError
	switch {
	case errors.Is(err, analysis.ErrInvalidArgument):
		return newErrorResponse(ctx, "invalid_argument", "Invalid analysis argument", nil), http.StatusBadRequest
	case errors.Is(err, analysis.ErrSymbolNotFound):
		return newErrorResponse(ctx, "symbol_not_found", "Symbol is unknown or inactive", nil), http.StatusNotFound
	case errors.As(err, &insufficient):
		return newErrorResponse(ctx, "insufficient_data", "Not enough closed candles for the requested period", map[string]any{"symbol": symbol, "criterion": insufficient.Criterion, "required": insufficient.Required, "available": insufficient.Available}), http.StatusConflict
	case errors.Is(err, analysis.ErrMarketDataUnavailable):
		return newErrorResponse(ctx, "market_data_unavailable", "Market data is unavailable", nil), http.StatusServiceUnavailable
	case errors.Is(err, analysis.ErrMarketCapUnavailable):
		return newErrorResponse(ctx, "market_cap_unavailable", "Market capitalization data is unavailable", nil), http.StatusServiceUnavailable
	case errors.As(err, &unresolved):
		return newErrorResponse(ctx, unresolved.Code, unresolved.Message, map[string]any{"symbol": symbol}), http.StatusUnprocessableEntity
	default:
		return newErrorResponse(ctx, "internal_error", "Internal server error", nil), http.StatusInternalServerError
	}
}

func newErrorResponse(ctx context.Context, code, message string, details any) ErrorResponse {
	return ErrorResponse{Error: APIError{Code: APIErrorCode(code), Message: message, Details: details}, RequestId: RequestIdentifier(ctx)}
}

func writeAPIError(response http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(response, status, ErrorResponse{Error: APIError{Code: APIErrorCode(code), Message: message, Details: details}, RequestId: response.Header().Get("X-Request-ID")})
}

func writeJSON(response http.ResponseWriter, status int, body any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(body)
}

func roundPercentage(value float64) float64 { return math.Round(value*10_000) / 10_000 }

func responseEvaluations(evaluations []analysis.Evaluation) []Evaluation {
	items := make([]Evaluation, len(evaluations))
	for i, evaluation := range evaluations {
		metrics := make(map[string]float64, len(evaluation.Metrics))
		for name, value := range evaluation.Metrics {
			metrics[name] = value
		}
		items[i] = Evaluation{Key: evaluation.Key, Name: evaluation.Name, Label: evaluation.Label, Matched: evaluation.Matched, Metrics: metrics, CandleCount: evaluation.CandleCount, From: evaluation.From.UTC(), To: evaluation.To.UTC()}
	}
	return items
}

// Market Scan retains its presentation rounding. Instrument Analysis carries
// original metrics so derived recommendations can be calculated before rounding.
func responseMarketScanEvaluations(evaluations []analysis.Evaluation) []Evaluation {
	items := responseEvaluations(evaluations)
	for index := range items {
		for name, value := range items[index].Metrics {
			if strings.HasSuffix(name, "_percent") {
				items[index].Metrics[name] = roundPercentage(value)
			}
		}
	}
	return items
}

func responseWarnings(warnings []analysis.Warning) []Warning {
	result := make([]Warning, len(warnings))
	for i, warning := range warnings {
		result[i] = Warning{Code: warning.Code, Message: warning.Message}
	}
	return result
}
