package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/analysis/criteria/volatility"
	"crypto-scanner/internal/auth"
	authtelegram "crypto-scanner/internal/auth/telegram"
	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/logging"
)

const analysisInitData = "auth_date=1785902400&query_id=AAHdF6IQAAAAAN0XogcAAAAA&user=%7B%22id%22%3A424242%2C%22first_name%22%3A%22Alice%22%2C%22username%22%3A%22alice%22%7D&hash=3787d0e46c1919cd293ec89f766ac33375446dbd7311acc07e422fecfc07812b"
const analysisBody = `{"criteria":[{"key":"daily_volatility","name":"volatility","label":"Daily Volatility","parameters":{"unit":"days","period":2,"percentile":50,"minimum_range_percent":0}}]}`

func TestAuthenticatedUserCanAnalyzeOneInstrument(t *testing.T) {
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	store := httpStore{instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}}, candles: map[int64][]market.Candle{1: httpCandles(start, 2, 24*time.Hour, 2)}}
	response := analysisRequestTo(t, newAnalysisHTTPHandler(store), "/api/v1/analysis/instruments/BTCUSDT", analysisBody)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	var body symbolAnalysisResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Symbol != "BTCUSDT" || !body.Matched || len(body.Evaluations) != 1 {
		t.Fatalf("response = %+v", body)
	}
	evaluation := body.Evaluations[0]
	if evaluation.Key != "daily_volatility" || evaluation.Name != "volatility" || evaluation.Label != "Daily Volatility" || !evaluation.Matched || evaluation.CandleCount != 2 || evaluation.Metrics["range_percent"] != 2 || !evaluation.From.Equal(start.Add(-24*time.Hour)) || !evaluation.To.Equal(start) {
		t.Fatalf("evaluation = %+v", evaluation)
	}
}

func TestInstrumentAnalysisPreservesRangePrecisionForGridSteps(t *testing.T) {
	for _, test := range []struct {
		name         string
		rangePercent float64
		scanRange    float64
	}{
		// 0.02469 / 2 displays as 0.0123%; rounding first gives 0.0124%.
		{"rounding changes displayed step", 0.02469, 0.0247},
		{"small positive range", 0.00001234, 0},
		{"genuine zero", 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
			store := httpStore{instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}}, candles: map[int64][]market.Candle{1: httpCandles(start, 2, 24*time.Hour, test.rangePercent)}}
			handler := newAnalysisHTTPHandler(store)
			response := analysisRequestTo(t, handler, "/api/v1/analysis/instruments/BTCUSDT", analysisBody)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
			}
			var body symbolAnalysisResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Evaluations) != 1 {
				t.Fatalf("evaluations = %+v", body.Evaluations)
			}
			if got := body.Evaluations[0].Metrics["range_percent"]; math.Abs(got-test.rangePercent) > 1e-12 {
				t.Fatalf("range = %.12g, want %.12g", got, test.rangePercent)
			}

			response = analysisRequestTo(t, handler, "/api/v1/analysis/market", analysisBody)
			var scan marketAnalysisResponse
			if response.Code != http.StatusOK {
				t.Fatalf("scan status = %d; body = %s", response.Code, response.Body.String())
			}
			if err := json.Unmarshal(response.Body.Bytes(), &scan); err != nil {
				t.Fatal(err)
			}
			if len(scan.Items) != 1 || len(scan.Items[0].Evaluations) != 1 || scan.Items[0].Evaluations[0].Metrics["range_percent"] != test.scanRange {
				t.Fatalf("scan presentation changed: %+v", scan.Items)
			}
		})
	}
}

func TestAnalysisCorrelatesRepeatedCriterionTypesByInstanceIdentity(t *testing.T) {
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	store := httpStore{instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}}, candles: map[int64][]market.Candle{1: {httpCandle(start, 2)}}}
	bodyRequest := `{"criteria":[{"key":"daily_volatility","name":"volatility","label":"Daily Volatility","parameters":{"unit":"days","period":1,"percentile":50,"minimum_range_percent":0}},{"key":"hourly_volatility","name":"volatility","label":"Hourly Volatility","parameters":{"unit":"hours","period":1,"percentile":50,"minimum_range_percent":0}}]}`
	response := analysisRequestTo(t, newAnalysisHTTPHandler(store), "/api/v1/analysis/instruments/BTCUSDT", bodyRequest)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	var body symbolAnalysisResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Evaluations) != 2 || body.Evaluations[0].Key != "daily_volatility" || body.Evaluations[0].Label != "Daily Volatility" || body.Evaluations[1].Key != "hourly_volatility" || body.Evaluations[1].Label != "Hourly Volatility" {
		t.Fatalf("evaluations = %+v", body.Evaluations)
	}
}

func TestAuthenticatedUserCanSearchMarket(t *testing.T) {
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	store := httpStore{
		instruments: []market.Instrument{{ID: 1, Symbol: "ZZZUSDT"}, {ID: 2, Symbol: "AAAUSDT"}, {ID: 3, Symbol: "NEWUSDT"}},
		candles: map[int64][]market.Candle{
			1: httpCandles(start, 2, 24*time.Hour, 9.43814),
			2: httpCandles(start, 2, 24*time.Hour, 4),
			3: {},
		},
	}
	bodyRequest := `{"criteria":[{"key":"volatility","name":"volatility","label":"Volatility","parameters":{"unit":"days","period":2,"percentile":75,"minimum_range_percent":4}}]}`
	response := analysisRequestTo(t, newAnalysisHTTPHandler(store), "/api/v1/analysis/market", bodyRequest)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	var body marketAnalysisResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.MatchedCount != 2 || body.AnalyzedCount != 2 || body.InsufficientDataCount != 1 || len(body.Items) != 2 {
		t.Fatalf("response = %+v", body)
	}
	if body.Items[0].Symbol != "ZZZUSDT" || body.Items[1].Symbol != "AAAUSDT" {
		t.Fatalf("items = %+v", body.Items)
	}
	if len(body.Items[0].Evaluations) != 1 || len(body.Items[1].Evaluations) != 1 || !body.Items[0].Matched || !body.Items[1].Matched {
		t.Fatalf("items = %+v", body.Items)
	}
	first, second := body.Items[0].Evaluations[0], body.Items[1].Evaluations[0]
	if first.Key != "volatility" || first.Name != "volatility" || first.Label != "Volatility" || !first.Matched || first.CandleCount != 2 || first.Metrics["range_percent"] != 9.4381 || second.Key != "volatility" || second.Name != "volatility" || second.Label != "Volatility" || !second.Matched || second.CandleCount != 2 || second.Metrics["range_percent"] != 4 {
		t.Fatalf("evaluations = %+v", body.Items)
	}
}
func TestAuthenticatedUserCanRequestSortedLimitedMarketScan(t *testing.T) {
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	selected := analysis.Selection{}
	store := &httpStore{
		rankedInstruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}},
		candles:           map[int64][]market.Candle{1: {httpCandle(start, 2)}},
		selected:          &selected,
	}
	body := `{"criteria":[{"key":"daily_volatility","name":"volatility","label":"Daily Volatility","parameters":{"unit":"days","period":1,"percentile":50,"minimum_range_percent":0}},{"key":"market_cap","name":"market_cap","label":"Market Cap","parameters":{}}],"limit":10,"sort":{"field":"market_cap_usd","direction":"desc"}}`
	response := analysisRequestTo(t, newAnalysisHTTPHandler(store, httpMarketCapFactory{}), "/api/v1/analysis/market", body)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	var result marketAnalysisResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if selected.Limit != 10 || selected.SortFact != analysis.SelectionFactMarketCapUSD || selected.SortDirection != "desc" || len(result.Items) != 1 || result.Items[0].Symbol != "BTCUSDT" {
		t.Fatalf("selection=%+v result=%+v", selected, result)
	}
}

func TestAnalysisRejectsMalformedAndUnknownJSON(t *testing.T) {
	for _, body := range []string{"{", `{"criteria":[],"extra":true}`, `{"criteria":[],"exclude_stablecoins":false}`, `{"criteria":[{"key":"volatility","name":"volatility","label":"Volatility","parameters":{},"extra":true}]}`, `{"criteria":[],"sort":{"field":"market_cap_usd","direction":"desc","extra":true}}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/market", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "tma "+analysisInitData)
		res := httptest.NewRecorder()
		newAnalysisHTTPHandler(httpStore{}).ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d", body, res.Code)
		}
		var envelope struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("decode %q: %v", res.Body.String(), err)
		}
		if envelope.Error.Message != "Invalid analysis argument" {
			t.Fatalf("message = %q", envelope.Error.Message)
		}
	}
}

func TestAnalysisSchemaValidationRejectsRequestsBeforeService(t *testing.T) {
	validCriteria := `[{"key":"volatility","name":"volatility","label":"Volatility","parameters":{}}]`
	for _, test := range []struct {
		name string
		path string
		body string
	}{
		{name: "missing market criteria", path: "/api/v1/analysis/market", body: `{}`},
		{name: "empty market criteria", path: "/api/v1/analysis/market", body: `{"criteria":[]}`},
		{name: "missing instrument criteria", path: "/api/v1/analysis/instruments/BTCUSDT", body: `{}`},
		{name: "empty instrument criteria", path: "/api/v1/analysis/instruments/BTCUSDT", body: `{"criteria":[]}`},
		{name: "market limit below contract", path: "/api/v1/analysis/market", body: `{"criteria":` + validCriteria + `,"limit":-1}`},
		{name: "market limit exceeds contract", path: "/api/v1/analysis/market", body: `{"criteria":` + validCriteria + `,"limit":101}`},
		{name: "instrument rejects market limit", path: "/api/v1/analysis/instruments/BTCUSDT", body: `{"criteria":` + validCriteria + `,"limit":1}`},
		{name: "instrument rejects market sort", path: "/api/v1/analysis/instruments/BTCUSDT", body: `{"criteria":` + validCriteria + `,"sort":{"field":"market_cap_usd","direction":"desc"}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &countingAnalysis{}
			handler := httpapi.NewWithAuthentication(logging.New(io.Discard, "error"), httpapi.Dependencies{Readiness: readinessStub{}, Analysis: service}, httpapi.Options{}, passThrough)
			response := analysisRequestTo(t, handler, test.path, test.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var envelope struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Error.Message != "Invalid analysis argument" {
				t.Fatalf("message = %q", envelope.Error.Message)
			}
			if service.symbolCalls != 0 || service.searchCalls != 0 {
				t.Fatalf("schema-invalid request reached service: %+v", service)
			}
		})
	}
}

func TestMarketAnalysisPreservesOmittedAndZeroLimitWithOptionalSort(t *testing.T) {
	criteria := `[{"key":"market_cap","name":"market_cap","label":"Market Cap","parameters":{}}]`
	for _, test := range []struct {
		name      string
		body      string
		wantLimit int
		wantSort  *analysis.SearchSort
	}{
		{name: "omitted limit and sort", body: `{"criteria":` + criteria + `}`, wantLimit: 0},
		{name: "zero limit", body: `{"criteria":` + criteria + `,"limit":0}`, wantLimit: 0},
		{name: "sort without limit", body: `{"criteria":` + criteria + `,"sort":{"field":"market_cap_usd","direction":"desc"}}`, wantLimit: 0, wantSort: &analysis.SearchSort{Field: "market_cap_usd", Direction: "desc"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &countingAnalysis{}
			handler := httpapi.NewWithAuthentication(logging.New(io.Discard, "error"), httpapi.Dependencies{Readiness: readinessStub{}, Analysis: service}, httpapi.Options{}, passThrough)
			response := analysisRequestTo(t, handler, "/api/v1/analysis/market", test.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if service.searchCalls != 1 || service.symbolCalls != 0 {
				t.Fatalf("service calls = %+v", service)
			}
			if service.searchRequest.Limit != test.wantLimit || !reflect.DeepEqual(service.searchRequest.Sort, test.wantSort) {
				t.Fatalf("search request = %+v, want limit=%d sort=%+v", service.searchRequest, test.wantLimit, test.wantSort)
			}
		})
	}
}

func TestAnalysisEndpointsRequireAuthentication(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/market", bytes.NewBufferString(analysisBody))
	res := httptest.NewRecorder()
	newAnalysisHTTPHandler(httpStore{}).ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatal(res.Code)
	}
}

func TestAnalysisPublicEndpointsReturnCanonicalErrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		store   httpStore
		path    string
		body    string
		status  int
		code    string
		details map[string]any
	}{
		{
			name:   "unknown symbol",
			store:  httpStore{instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}}},
			path:   "/api/v1/analysis/instruments/UNKNOWNUSDT",
			body:   analysisBody,
			status: http.StatusNotFound,
			code:   "symbol_not_found",
		},
		{
			name:    "insufficient history",
			store:   httpStore{instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}}, candles: map[int64][]market.Candle{1: {}}},
			path:    "/api/v1/analysis/instruments/BTCUSDT",
			body:    analysisBody,
			status:  http.StatusConflict,
			code:    "insufficient_data",
			details: map[string]any{"symbol": "BTCUSDT", "criterion": "volatility", "required": float64(2), "available": float64(0)},
		},
		{
			name:   "market data unavailable",
			store:  httpStore{syncState: &market.SyncState{}},
			path:   "/api/v1/analysis/market",
			body:   analysisBody,
			status: http.StatusServiceUnavailable,
			code:   "market_data_unavailable",
		},
		{
			name:   "invalid criterion parameters",
			store:  httpStore{},
			path:   "/api/v1/analysis/market",
			body:   `{"criteria":[{"key":"volatility","name":"volatility","label":"Volatility","parameters":{"unit":"weeks","period":2,"percentile":50,"minimum_range_percent":0}}]}`,
			status: http.StatusBadRequest,
			code:   "invalid_argument",
		},
		{
			name:   "unknown criterion",
			store:  httpStore{},
			path:   "/api/v1/analysis/market",
			body:   `{"criteria":[{"key":"unknown","name":"unknown","label":"Unknown","parameters":{}}]}`,
			status: http.StatusBadRequest,
			code:   "invalid_argument",
		},
		{
			name:   "unknown criterion parameter",
			store:  httpStore{},
			path:   "/api/v1/analysis/market",
			body:   `{"criteria":[{"key":"volatility","name":"volatility","label":"Volatility","parameters":{"unit":"days","period":2,"percentile":50,"minimum_range_percent":0,"unknown":true}}]}`,
			status: http.StatusBadRequest,
			code:   "invalid_argument",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := analysisRequestTo(t, newAnalysisHTTPHandler(test.store), test.path, test.body)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}

			var envelope struct {
				Error struct {
					Code    string         `json:"code"`
					Message string         `json:"message"`
					Details map[string]any `json:"details"`
				} `json:"error"`
				RequestID string `json:"request_id"`
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode error envelope: %v", err)
			}
			if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
				t.Fatalf("decode raw error envelope: %v", err)
			}
			if len(raw) != 2 || raw["error"] == nil || raw["request_id"] == nil {
				t.Fatalf("error envelope = %s, want only error and request_id", response.Body.String())
			}
			if envelope.Error.Code != test.code || envelope.Error.Message == "" || !reflect.DeepEqual(envelope.Error.Details, test.details) {
				t.Fatalf("error envelope = %+v, want code %q and details %#v", envelope, test.code, test.details)
			}
			if requestID := response.Header().Get("X-Request-ID"); requestID == "" || requestID != envelope.RequestID {
				t.Fatalf("X-Request-ID = %q, request_id = %q", requestID, envelope.RequestID)
			}
		})
	}
}

func analysisRequestTo(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "tma "+analysisInitData)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
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

type symbolAnalysisResponse struct {
	Symbol      string               `json:"symbol"`
	Matched     bool                 `json:"matched"`
	Evaluations []evaluationResponse `json:"evaluations"`
}

type marketAnalysisResponse struct {
	MatchedCount          int `json:"matched_count"`
	AnalyzedCount         int `json:"analyzed_count"`
	InsufficientDataCount int `json:"insufficient_data_count"`
	Items                 []struct {
		Symbol      string               `json:"symbol"`
		Matched     bool                 `json:"matched"`
		Evaluations []evaluationResponse `json:"evaluations"`
	} `json:"items"`
}

const fixtureBotToken = "123456789:AAExampleBotTokenForDeterministicTests"

type countingAnalysis struct {
	symbolCalls   int
	searchCalls   int
	searchRequest analysis.SearchRequest
}

func (service *countingAnalysis) AnalyzeSymbol(context.Context, analysis.SymbolRequest) (analysis.SymbolResult, error) {
	service.symbolCalls++
	return analysis.SymbolResult{}, analysis.ErrInvalidArgument
}

func (service *countingAnalysis) Search(_ context.Context, request analysis.SearchRequest) (analysis.SearchResult, error) {
	service.searchCalls++
	service.searchRequest = request
	return analysis.SearchResult{}, analysis.ErrInvalidArgument
}

type enabledUserStore struct{}

func (enabledUserStore) FindEnabledByTelegramID(context.Context, int64) (auth.User, error) {
	return auth.User{ID: 1, TelegramID: 424242, Enabled: true}, nil
}
func newAnalysisHTTPHandler(store analysis.Store, additionalFactories ...analysis.Factory) http.Handler {
	factories := append([]analysis.Factory{volatility.New()}, additionalFactories...)
	service, _ := analysis.NewService(store, nil, factories...)
	authenticator := authtelegram.New(enabledUserStore{}, fixtureBotToken, 15*time.Minute, authtelegram.Options{Now: func() time.Time { return time.Date(2026, 8, 5, 4, 10, 0, 0, time.UTC) }})
	return httpapi.New(logging.New(io.Discard, "error"), httpapi.Dependencies{Readiness: readinessStub{marketSync: true}, Analysis: service, Authenticator: authenticator}, httpapi.Options{})
}

type httpStore struct {
	candlesByInterval map[int64]map[string][]market.Candle
	instruments       []market.Instrument
	rankedInstruments []market.Instrument
	candles           map[int64][]market.Candle
	syncState         *market.SyncState
	selected          *analysis.Selection
}

func (s httpStore) GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error) {
	if s.syncState != nil {
		return *s.syncState, nil
	}
	now := time.Now()
	return market.SyncState{LastSucceededAt: &now}, nil
}
func (s httpStore) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	return s.instruments, nil
}
func (s httpStore) SelectActiveInstruments(_ context.Context, selection analysis.Selection) ([]market.Instrument, error) {
	if s.selected != nil {
		*s.selected = selection
	}
	items := s.instruments
	if selection.SortFact == analysis.SelectionFactMarketCapUSD && s.rankedInstruments != nil {
		items = s.rankedInstruments
	}
	if selection.Limit > 0 {
		items = items[:min(selection.Limit, len(items))]
	}
	return items, nil
}
func (s httpStore) ListHourlyPrices(context.Context, []int64, time.Time, time.Time) ([]market.HourlyPrice, error) {
	return nil, nil
}
func (s httpStore) ListLatestCandles(_ context.Context, instrumentIDs []int64, interval market.CandleInterval, _ int) (map[int64][]market.Candle, error) {
	result := make(map[int64][]market.Candle, len(instrumentIDs))
	for _, instrumentID := range instrumentIDs {
		result[instrumentID] = s.candles[instrumentID]
		if s.candlesByInterval != nil {
			result[instrumentID] = s.candlesByInterval[instrumentID][string(interval)]
		}
	}
	return result, nil
}

type httpMarketCapFactory struct{}

func (httpMarketCapFactory) Name() string { return "market_cap" }
func (httpMarketCapFactory) Build(map[string]any) (analysis.Criterion, error) {
	return httpMarketCapCriterion{}, nil
}

type httpMarketCapCriterion struct{}

func (httpMarketCapCriterion) Name() string                               { return "market_cap" }
func (httpMarketCapCriterion) Requirements() []analysis.CandleRequirement { return nil }
func (httpMarketCapCriterion) MinimumMarketCapUSD() float64               { return 0 }
func (httpMarketCapCriterion) Evaluate(context.Context, analysis.Input) (analysis.Evaluation, error) {
	return analysis.Evaluation{Matched: true, Metrics: map[string]float64{"market_cap_usd": 1}}, nil
}

func httpCandles(end time.Time, count int, step time.Duration, rangePercent float64) []market.Candle {
	candles := make([]market.Candle, count)
	for i := range candles {
		candles[i] = httpCandle(end.Add(time.Duration(i-count+1)*step), rangePercent)
	}
	return candles
}

func httpCandle(openTime time.Time, rangePercent float64) market.Candle {
	return market.Candle{OpenTime: openTime, Open: 100, High: 100 + rangePercent, Low: 100}
}
