package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/analysis/percentile"
	"crypto-scanner/internal/auth"
	authtelegram "crypto-scanner/internal/auth/telegram"
	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/logging"
)

const analysisInitData = "auth_date=1785902400&query_id=AAHdF6IQAAAAAN0XogcAAAAA&user=%7B%22id%22%3A424242%2C%22first_name%22%3A%22Alice%22%2C%22username%22%3A%22alice%22%7D&hash=3787d0e46c1919cd293ec89f766ac33375446dbd7311acc07e422fecfc07812b"

func TestEnabledTelegramUserCanRequestOneSymbolPercentile(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.July, 5, 0, 0, 0, 0, time.UTC)
	store := httpAnalysisStore{
		synchronized: true,
		instruments:  []market.Instrument{{ID: 7, Symbol: "BTCUSDT", Active: true}},
		candles: map[int64][]market.Candle{
			7: {
				{OpenTime: start, Open: 100, High: 102, Low: 100},
				{OpenTime: start.AddDate(0, 0, 1), Open: 100, High: 106, Low: 100},
			},
		},
	}
	service := analysis.NewService(store, percentile.New())
	authenticator := authtelegram.NewWithOptions(enabledUserStore{}, fixtureBotToken, 15*time.Minute, authtelegram.Options{
		Now: func() time.Time { return time.Date(2026, time.August, 5, 4, 10, 0, 0, time.UTC) },
	})
	handler := httpapi.New(
		logging.New(io.Discard, "error"), readinessStub{marketSync: true},
		service, authenticator,
	)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/analysis/percentile/BTCUSDT?period_days=2&percentile=50", nil)
	request.Header.Set("Authorization", "tma "+analysisInitData)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Symbol       string    `json:"symbol"`
		PeriodDays   int       `json:"period_days"`
		Percentile   float64   `json:"percentile"`
		RangePercent float64   `json:"range_percent"`
		CandleCount  int       `json:"candle_count"`
		From         time.Time `json:"from"`
		To           time.Time `json:"to"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Symbol != "BTCUSDT" || body.PeriodDays != 2 || body.Percentile != 50 || body.RangePercent != 4 || body.CandleCount != 2 {
		t.Fatalf("response = %+v", body)
	}
	if !body.From.Equal(start) || !body.To.Equal(start.AddDate(0, 0, 1)) {
		t.Fatalf("coverage = %s..%s", body.From, body.To)
	}
}

func TestEnabledTelegramUserCanSearchAllActiveInstruments(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.July, 5, 0, 0, 0, 0, time.UTC)
	store := httpAnalysisStore{
		synchronized: true,
		instruments: []market.Instrument{
			{ID: 1, Symbol: "ZZZUSDT", Active: true}, {ID: 2, Symbol: "AAAUSDT", Active: true},
			{ID: 3, Symbol: "NEWUSDT", Active: true},
		},
		candles: map[int64][]market.Candle{
			1: {{OpenTime: start, Open: 100_000, High: 109_438.14, Low: 100_000}},
			2: {{OpenTime: start, Open: 100, High: 104, Low: 100}},
			3: {},
		},
	}
	handler := newAnalysisHTTPHandler(store)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/analysis/percentile?period_days=1&percentile=75&minimum_range_percent=4", nil)
	request.Header.Set("Authorization", "tma "+analysisInitData)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	var body struct {
		MatchedCount          int `json:"matched_count"`
		AnalyzedCount         int `json:"analyzed_count"`
		InsufficientDataCount int `json:"insufficient_data_count"`
		Items                 []struct {
			Symbol       string  `json:"symbol"`
			RangePercent float64 `json:"range_percent"`
			CandleCount  int     `json:"candle_count"`
		} `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.MatchedCount != 2 || body.AnalyzedCount != 2 || body.InsufficientDataCount != 1 || len(body.Items) != 2 {
		t.Fatalf("response counts/items = %+v", body)
	}
	if body.Items[0].Symbol != "ZZZUSDT" || body.Items[0].RangePercent != 9.4381 || body.Items[1].Symbol != "AAAUSDT" {
		t.Fatalf("response items = %+v", body.Items)
	}
}

func TestAnalysisErrorsUseCanonicalEnvelope(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.July, 5, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		store      httpAnalysisStore
		target     string
		wantStatus int
		wantCode   string
		wantDetail string
	}{
		{
			name: "unknown symbol", store: httpAnalysisStore{synchronized: true},
			target: "/api/v1/analysis/percentile/UNKNOWN?period_days=1&percentile=50", wantStatus: 404, wantCode: "symbol_not_found",
		},
		{
			name: "insufficient history",
			store: httpAnalysisStore{synchronized: true, instruments: []market.Instrument{{ID: 4, Symbol: "NEWUSDT", Active: true}}, candles: map[int64][]market.Candle{
				4: {{OpenTime: start, Open: 100, High: 101, Low: 100}},
			}},
			target: "/api/v1/analysis/percentile/NEWUSDT?period_days=2&percentile=50", wantStatus: 409, wantCode: "insufficient_data", wantDetail: `"required":2`,
		},
		{
			name: "no synchronized dataset", store: httpAnalysisStore{},
			target: "/api/v1/analysis/percentile?period_days=1&percentile=50&minimum_range_percent=0", wantStatus: 503, wantCode: "market_data_unavailable",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			request.Header.Set("Authorization", "tma "+analysisInitData)
			response := httptest.NewRecorder()
			newAnalysisHTTPHandler(test.store).ServeHTTP(response, request)
			assertAnalysisError(t, response, test.wantStatus, test.wantCode)
			if test.wantDetail != "" && !strings.Contains(response.Body.String(), test.wantDetail) {
				t.Fatalf("error body = %s, want %s", response.Body.String(), test.wantDetail)
			}
		})
	}
}

func TestAnalysisRejectsInvalidArgumentsBeforeReadingMarketData(t *testing.T) {
	t.Parallel()

	targets := []string{
		"/api/v1/analysis/percentile/BTCUSDT?percentile=50",
		"/api/v1/analysis/percentile/BTCUSDT?period_days=0&percentile=50",
		"/api/v1/analysis/percentile/BTCUSDT?period_days=1&percentile=101",
		"/api/v1/analysis/percentile/BTCUSDT?period_days=one&percentile=50",
		"/api/v1/analysis/percentile/BTCUSDT?period_days=1&period_days=2&percentile=50",
		"/api/v1/analysis/percentile?period_days=1&percentile=50&minimum_range_percent=-1",
		"/api/v1/analysis/percentile?period_days=1&percentile=50&minimum_range_percent=0&extra=true",
	}
	for _, target := range targets {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, target, nil)
			request.Header.Set("Authorization", "tma "+analysisInitData)
			response := httptest.NewRecorder()
			newAnalysisHTTPHandler(httpAnalysisStore{synchronized: true}).ServeHTTP(response, request)
			assertAnalysisError(t, response, http.StatusBadRequest, "invalid_argument")
		})
	}
}

func TestAnalysisEndpointsRequireTelegramAuthentication(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/analysis/percentile/BTCUSDT?period_days=1&percentile=50", nil)
	response := httptest.NewRecorder()
	newAnalysisHTTPHandler(httpAnalysisStore{synchronized: true}).ServeHTTP(response, request)
	assertAnalysisError(t, response, http.StatusUnauthorized, "unauthenticated")
}

const fixtureBotToken = "123456789:AAExampleBotTokenForDeterministicTests"

type enabledUserStore struct{}

func (enabledUserStore) FindEnabledByTelegramID(context.Context, int64) (auth.User, error) {
	return auth.User{ID: 1, TelegramID: 424242, Enabled: true}, nil
}

func newAnalysisHTTPHandler(store httpAnalysisStore) http.Handler {
	service := analysis.NewService(store, percentile.New())
	authenticator := authtelegram.NewWithOptions(enabledUserStore{}, fixtureBotToken, 15*time.Minute, authtelegram.Options{
		Now: func() time.Time { return time.Date(2026, time.August, 5, 4, 10, 0, 0, time.UTC) },
	})
	return httpapi.New(logging.New(io.Discard, "error"), readinessStub{marketSync: true}, service, authenticator)
}

func assertAnalysisError(t *testing.T, response *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, wantStatus, response.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != wantCode || body.RequestID == "" || response.Header().Get("X-Request-ID") != body.RequestID {
		t.Fatalf("error response = %+v headers=%v", body, response.Header())
	}
}

type httpAnalysisStore struct {
	synchronized bool
	instruments  []market.Instrument
	candles      map[int64][]market.Candle
}

func (stub httpAnalysisStore) GetSyncState(context.Context, market.SyncProfile) (market.SyncState, error) {
	state := market.SyncState{}
	if stub.synchronized {
		succeededAt := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
		state.LastSucceededAt = &succeededAt
	}
	return state, nil
}
func (stub httpAnalysisStore) ListActiveInstruments(context.Context) ([]market.Instrument, error) {
	return append([]market.Instrument(nil), stub.instruments...), nil
}
func (stub httpAnalysisStore) ListLatestCandles(_ context.Context, instrumentID int64, limit int) ([]market.Candle, error) {
	items := append([]market.Candle(nil), stub.candles[instrumentID]...)
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}
