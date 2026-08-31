package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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
	store := httpStore{instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}}, candles: map[int64][]market.Candle{1: {httpCandle(start, 2)}}}
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
	if evaluation.Key != "daily_volatility" || evaluation.Name != "volatility" || evaluation.Label != "Daily Volatility" || !evaluation.Matched || evaluation.CandleCount != 1 || evaluation.Metrics["range_percent"] != 2 || !evaluation.From.Equal(start) || !evaluation.To.Equal(start) {
		t.Fatalf("evaluation = %+v", evaluation)
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
			1: {httpCandle(start, 9.43814)}, 2: {httpCandle(start, 4)}, 3: {},
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
	if first.Key != "volatility" || first.Name != "volatility" || first.Label != "Volatility" || !first.Matched || first.CandleCount != 1 || first.Metrics["range_percent"] != 9.4381 || second.Key != "volatility" || second.Name != "volatility" || second.Label != "Volatility" || !second.Matched || second.CandleCount != 1 || second.Metrics["range_percent"] != 4 {
		t.Fatalf("evaluations = %+v", body.Items)
	}
}
func TestAnalysisRejectsMalformedAndUnknownJSON(t *testing.T) {
	for _, body := range []string{"{", `{"criteria":[],"extra":true}`, `{"criteria":[{"key":"volatility","name":"volatility","label":"Volatility","parameters":{},"extra":true}]}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/analysis/market", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "tma "+analysisInitData)
		res := httptest.NewRecorder()
		newAnalysisHTTPHandler(httpStore{}).ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d", body, res.Code)
		}
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

type enabledUserStore struct{}

func (enabledUserStore) FindEnabledByTelegramID(context.Context, int64) (auth.User, error) {
	return auth.User{ID: 1, TelegramID: 424242, Enabled: true}, nil
}
func newAnalysisHTTPHandler(store httpStore, additionalFactories ...analysis.Factory) http.Handler {
	factories := append([]analysis.Factory{volatility.New()}, additionalFactories...)
	service, _ := analysis.NewService(store, factories...)
	authenticator := authtelegram.NewWithOptions(enabledUserStore{}, fixtureBotToken, 15*time.Minute, authtelegram.Options{Now: func() time.Time { return time.Date(2026, 8, 5, 4, 10, 0, 0, time.UTC) }})
	return httpapi.New(logging.New(io.Discard, "error"), readinessStub{marketSync: true}, service, authenticator)
}

type httpStore struct {
	candlesByInterval map[int64]map[string][]market.Candle
	instruments       []market.Instrument
	candles           map[int64][]market.Candle
	syncState         *market.SyncState
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
func (s httpStore) ListLatestCandlesByInterval(_ context.Context, instrumentID int64, interval string, _ int) ([]market.Candle, error) {
	if s.candlesByInterval != nil {
		return s.candlesByInterval[instrumentID][interval], nil
	}
	return s.candles[instrumentID], nil
}
func httpCandle(openTime time.Time, rangePercent float64) market.Candle {
	return market.Candle{OpenTime: openTime, Open: 100, High: 100 + rangePercent, Low: 100}
}
