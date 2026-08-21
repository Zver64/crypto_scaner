package httpapi

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"crypto-scanner/internal/analysis"
)

func analyzeSymbol(service Analysis) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		unit, period, percentile, ok := analysisParameters(request.URL.Query(), false)
		if !ok {
			writeAPIError(response, http.StatusBadRequest, "invalid_argument", "Invalid analysis argument", nil)
			return
		}
		result, err := service.AnalyzeSymbol(request.Context(), analysis.SymbolRequest{
			Symbol: request.PathValue("symbol"), Unit: unit, Period: period, Percentile: percentile,
		})
		if err != nil {
			writeAnalysisError(response, err, request.PathValue("symbol"))
			return
		}
		writeJSON(response, http.StatusOK, symbolResponse{
			Symbol: result.Symbol, Unit: string(unit), Period: period, Percentile: percentile,
			RangePercent: roundPercentage(result.RangePercent), CandleCount: result.CandleCount,
			From: result.From.UTC(), To: result.To.UTC(),
		})
	}
}

func searchMarket(service Analysis) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		unit, period, percentile, ok := analysisParameters(request.URL.Query(), true)
		minimum, minimumOK := oneFloat(request.URL.Query(), "minimum_range_percent")
		if !ok || !minimumOK {
			writeAPIError(response, http.StatusBadRequest, "invalid_argument", "Invalid analysis argument", nil)
			return
		}
		result, err := service.Search(request.Context(), analysis.SearchRequest{
			Unit: unit, Period: period, Percentile: percentile, MinimumRangePercent: minimum,
		})
		if err != nil {
			writeAnalysisError(response, err, "")
			return
		}
		items := make([]searchItemResponse, len(result.Items))
		for index, item := range result.Items {
			items[index] = searchItemResponse{
				Symbol: item.Symbol, RangePercent: roundPercentage(item.RangePercent), CandleCount: item.CandleCount,
			}
		}
		writeJSON(response, http.StatusOK, searchResponse{
			Unit: string(unit), Period: period, Percentile: percentile, MinimumRangePercent: minimum,
			MatchedCount: result.MatchedCount, AnalyzedCount: result.AnalyzedCount,
			InsufficientDataCount: result.InsufficientDataCount, Items: items,
		})
	}
}

func analysisParameters(values url.Values, search bool) (analysis.Unit, int, float64, bool) {
	wantKeys := 3
	if search {
		wantKeys = 4
	}
	if len(values) != wantKeys {
		return "", 0, 0, false
	}
	unitValue, ok := oneString(values, "unit")
	unit := analysis.Unit(unitValue)
	if !ok || (unit != analysis.UnitDays && unit != analysis.UnitHours) {
		return "", 0, 0, false
	}
	period, ok := oneInt(values, "period")
	if !ok {
		return "", 0, 0, false
	}
	percentile, ok := oneFloat(values, "percentile")
	if !ok {
		return "", 0, 0, false
	}
	return unit, period, percentile, true
}

func oneString(values url.Values, name string) (string, bool) {
	items, ok := values[name]
	if !ok || len(items) != 1 || items[0] == "" {
		return "", false
	}
	return items[0], true
}

func oneInt(values url.Values, name string) (int, bool) {
	items, ok := values[name]
	if !ok || len(items) != 1 || items[0] == "" {
		return 0, false
	}
	value, err := strconv.Atoi(items[0])
	return value, err == nil
}

func oneFloat(values url.Values, name string) (float64, bool) {
	items, ok := values[name]
	if !ok || len(items) != 1 || items[0] == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(items[0], 64)
	return value, err == nil
}

func writeAnalysisError(response http.ResponseWriter, err error, symbol string) {
	var insufficient *analysis.InsufficientHistoryError
	switch {
	case errors.Is(err, analysis.ErrInvalidArgument):
		writeAPIError(response, http.StatusBadRequest, "invalid_argument", "Invalid analysis argument", nil)
	case errors.Is(err, analysis.ErrSymbolNotFound):
		writeAPIError(response, http.StatusNotFound, "symbol_not_found", "Symbol is unknown or inactive", nil)
	case errors.As(err, &insufficient):
		writeAPIError(response, http.StatusConflict, "insufficient_data", "Not enough closed candles for the requested period", map[string]any{
			"symbol": symbol, "required": insufficient.Required, "available": insufficient.Available,
		})
	case errors.Is(err, analysis.ErrMarketDataUnavailable):
		writeAPIError(response, http.StatusServiceUnavailable, "market_data_unavailable", "Market data is unavailable", nil)
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

type symbolResponse struct {
	Symbol       string    `json:"symbol"`
	Unit         string    `json:"unit"`
	Period       int       `json:"period"`
	Percentile   float64   `json:"percentile"`
	RangePercent float64   `json:"range_percent"`
	CandleCount  int       `json:"candle_count"`
	From         time.Time `json:"from"`
	To           time.Time `json:"to"`
}

type searchItemResponse struct {
	Symbol       string  `json:"symbol"`
	RangePercent float64 `json:"range_percent"`
	CandleCount  int     `json:"candle_count"`
}

type searchResponse struct {
	Unit                  string               `json:"unit"`
	Period                int                  `json:"period"`
	Percentile            float64              `json:"percentile"`
	MinimumRangePercent   float64              `json:"minimum_range_percent"`
	MatchedCount          int                  `json:"matched_count"`
	AnalyzedCount         int                  `json:"analyzed_count"`
	InsufficientDataCount int                  `json:"insufficient_data_count"`
	Items                 []searchItemResponse `json:"items"`
}
