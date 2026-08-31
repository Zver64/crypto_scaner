package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"crypto-scanner/internal/analysis"
)

const maxAnalysisRequestBody = 1 << 20

type analysisRequest struct {
	Criteria []criterionRequest `json:"criteria"`
}

type criterionRequest struct {
	Key        string         `json:"key"`
	Name       string         `json:"name"`
	Label      string         `json:"label"`
	Parameters map[string]any `json:"parameters"`
}

func (request analysisRequest) criterionConfigs() []analysis.CriterionConfig {
	configs := make([]analysis.CriterionConfig, len(request.Criteria))
	for i, criterion := range request.Criteria {
		configs[i] = analysis.CriterionConfig{Key: criterion.Key, Name: criterion.Name, Label: criterion.Label, Parameters: criterion.Parameters}
	}
	return configs
}

func analyzeSymbol(service Analysis) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		body, ok := decodeAnalysisRequest(response, request)
		if !ok {
			return
		}
		result, err := service.AnalyzeSymbol(request.Context(), analysis.SymbolRequest{Symbol: request.PathValue("symbol"), Criteria: body.criterionConfigs()})
		if err != nil {
			writeAnalysisError(response, err, request.PathValue("symbol"))
			return
		}
		writeJSON(response, http.StatusOK, symbolResponse{Symbol: result.Symbol, Matched: result.Matched, Evaluations: responseEvaluations(result.Evaluations), Warnings: responseWarnings(result.Warnings)})
	}
}

func searchMarket(service Analysis) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		body, ok := decodeAnalysisRequest(response, request)
		if !ok {
			return
		}
		result, err := service.Search(request.Context(), analysis.SearchRequest{Criteria: body.criterionConfigs()})
		if err != nil {
			writeAnalysisError(response, err, "")
			return
		}
		items := make([]searchItemResponse, len(result.Items))
		for i, item := range result.Items {
			items[i] = searchItemResponse{Symbol: item.Symbol, Matched: item.Matched, Evaluations: responseEvaluations(item.Evaluations)}
		}
		unresolved := make([]unresolvedResponse, len(result.Unresolved))
		for i, item := range result.Unresolved {
			unresolved[i] = unresolvedResponse{Symbol: item.Symbol, Code: item.Code, Message: item.Message}
		}
		writeJSON(response, http.StatusOK, searchResponse{MatchedCount: result.MatchedCount, AnalyzedCount: result.AnalyzedCount, InsufficientDataCount: result.InsufficientDataCount, Items: items, Unresolved: unresolved, Warnings: responseWarnings(result.Warnings)})
	}
}

func decodeAnalysisRequest(response http.ResponseWriter, request *http.Request) (analysisRequest, bool) {
	request.Body = http.MaxBytesReader(response, request.Body, maxAnalysisRequestBody)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var body analysisRequest
	if decoder.Decode(&body) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		writeAPIError(response, http.StatusBadRequest, "invalid_argument", "Invalid analysis argument", nil)
		return analysisRequest{}, false
	}
	return body, true
}

func writeAnalysisError(response http.ResponseWriter, err error, symbol string) {
	var insufficient *analysis.InsufficientHistoryError
	var unresolved *analysis.UnresolvedError
	switch {
	case errors.Is(err, analysis.ErrInvalidArgument):
		writeAPIError(response, http.StatusBadRequest, "invalid_argument", "Invalid analysis argument", nil)
	case errors.Is(err, analysis.ErrSymbolNotFound):
		writeAPIError(response, http.StatusNotFound, "symbol_not_found", "Symbol is unknown or inactive", nil)
	case errors.As(err, &insufficient):
		writeAPIError(response, http.StatusConflict, "insufficient_data", "Not enough closed candles for the requested period", map[string]any{"symbol": symbol, "criterion": insufficient.Criterion, "required": insufficient.Required, "available": insufficient.Available})
	case errors.Is(err, analysis.ErrMarketDataUnavailable):
		writeAPIError(response, http.StatusServiceUnavailable, "market_data_unavailable", "Market data is unavailable", nil)
	case errors.Is(err, analysis.ErrMarketCapUnavailable):
		writeAPIError(response, http.StatusServiceUnavailable, "market_cap_unavailable", "Market capitalization data is unavailable", nil)
	case errors.As(err, &unresolved):
		writeAPIError(response, http.StatusUnprocessableEntity, unresolved.Code, unresolved.Message, map[string]any{"symbol": symbol})
	default:
		writeAPIError(response, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
	}
}

func writeAPIError(response http.ResponseWriter, status int, code, message string, details any) {
	type errorBody struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details any    `json:"details,omitempty"`
	}
	writeJSON(response, status, struct {
		Error     errorBody `json:"error"`
		RequestID string    `json:"request_id"`
	}{Error: errorBody{Code: code, Message: message, Details: details}, RequestID: response.Header().Get("X-Request-ID")})
}
func writeJSON(response http.ResponseWriter, status int, body any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(body)
}
func roundPercentage(value float64) float64 { return math.Round(value*10_000) / 10_000 }

func responseEvaluations(evaluations []analysis.Evaluation) []evaluationResponse {
	items := make([]evaluationResponse, len(evaluations))
	for i, evaluation := range evaluations {
		metrics := make(map[string]float64, len(evaluation.Metrics))
		for name, value := range evaluation.Metrics {
			metrics[name] = value
			if strings.HasSuffix(name, "_percent") {
				metrics[name] = roundPercentage(value)
			}
		}
		items[i] = evaluationResponse{Key: evaluation.Key, Name: evaluation.Name, Label: evaluation.Label, Matched: evaluation.Matched, Metrics: metrics, CandleCount: evaluation.CandleCount, From: evaluation.From.UTC(), To: evaluation.To.UTC()}
	}
	return items
}

type evaluationResponse struct {
	Key         string             `json:"key"`
	Name        string             `json:"name"`
	Label       string             `json:"label"`
	Matched     bool               `json:"matched"`
	Metrics     map[string]float64 `json:"metrics"`
	CandleCount int                `json:"candle_count"`
	From        time.Time          `json:"from"`
	To          time.Time          `json:"to"`
}
type symbolResponse struct {
	Symbol      string               `json:"symbol"`
	Matched     bool                 `json:"matched"`
	Evaluations []evaluationResponse `json:"evaluations"`
	Warnings    []warningResponse    `json:"warnings"`
}
type searchItemResponse struct {
	Symbol      string               `json:"symbol"`
	Matched     bool                 `json:"matched"`
	Evaluations []evaluationResponse `json:"evaluations"`
}
type searchResponse struct {
	MatchedCount          int                  `json:"matched_count"`
	AnalyzedCount         int                  `json:"analyzed_count"`
	InsufficientDataCount int                  `json:"insufficient_data_count"`
	Items                 []searchItemResponse `json:"items"`
	Unresolved            []unresolvedResponse `json:"unresolved"`
	Warnings              []warningResponse    `json:"warnings"`
}
type unresolvedResponse struct {
	Symbol  string `json:"symbol"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type warningResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func responseWarnings(warnings []analysis.Warning) []warningResponse {
	result := make([]warningResponse, len(warnings))
	for i, warning := range warnings {
		result[i] = warningResponse{Code: warning.Code, Message: warning.Message}
	}
	return result
}
