package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"crypto-scanner/internal/httpapi"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/logging"
)

func TestCandleHistoryReturnsChronologicalKeysetPage(t *testing.T) {
	oldest := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	history := &candleHistoryStub{page: market.CandlePage{HasMore: true, Candles: []market.Candle{
		{OpenTime: oldest, CloseTime: oldest.Add(time.Hour - time.Millisecond), Open: 1, High: 3, Low: .5, Close: 2},
		{OpenTime: oldest.Add(time.Hour), CloseTime: oldest.Add(2*time.Hour - time.Millisecond), Open: 2, High: 4, Low: 1, Close: 3},
	}}}
	handler := httpapi.New(logging.New(io.Discard, "error"), readinessStub{}, unavailableAnalysis{}, history, passThroughAuthenticator{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/instruments/btcusdt/candles?interval=1h&limit=2", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Symbol     string `json:"symbol"`
		Interval   string `json:"interval"`
		HasMore    bool   `json:"has_more"`
		NextBefore string `json:"next_before"`
		Candles    []struct {
			OpenTime time.Time `json:"open_time"`
		} `json:"candles"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Symbol != "BTCUSDT" || body.Interval != "1h" || !body.HasMore || body.NextBefore != oldest.Format(time.RFC3339) || len(body.Candles) != 2 || !body.Candles[0].OpenTime.Equal(oldest) {
		t.Fatalf("response = %+v", body)
	}
	if history.interval != market.IntervalHour || history.limit != 2 || history.before != nil {
		t.Fatalf("store request = %+v", history)
	}
}

func TestCandleHistoryValidationPreservesCanonicalErrors(t *testing.T) {
	history := &candleHistoryStub{}
	handler := httpapi.New(logging.New(io.Discard, "error"), readinessStub{}, unavailableAnalysis{}, history, passThroughAuthenticator{})
	for _, test := range []struct {
		target  string
		message string
	}{
		{target: "/api/v1/instruments/BTCUSDT/candles?interval=4h", message: "Unsupported candle interval"},
		{target: "/api/v1/instruments/BTCUSDT/candles?interval=1d&limit=501", message: "Invalid candle page limit"},
		{target: "/api/v1/instruments/BTCUSDT/candles?interval=1M&before=yesterday", message: "Invalid candle cursor"},
		{target: "/api/v1/instruments/%20/candles?interval=1h", message: "Symbol is required"},
	} {
		t.Run(test.message, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.target, nil))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var envelope struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode %q: %v", response.Body.String(), err)
			}
			if envelope.Error.Message != test.message {
				t.Fatalf("message = %q, want %q", envelope.Error.Message, test.message)
			}
			if history.interval != "" || history.limit != 0 || history.before != nil {
				t.Fatalf("schema-invalid request reached candle store: %+v", history)
			}
		})
	}
}

type candleHistoryStub struct {
	page     market.CandlePage
	interval market.CandleInterval
	before   *time.Time
	limit    int
}

func (stub *candleHistoryStub) GetActiveInstrumentBySymbol(_ context.Context, symbol string) (market.Instrument, error) {
	return market.Instrument{ID: 1, Symbol: symbol, Active: true}, nil
}

func (stub *candleHistoryStub) ListCandlePage(_ context.Context, _ int64, interval market.CandleInterval, before *time.Time, limit int) (market.CandlePage, error) {
	stub.interval, stub.before, stub.limit = interval, before, limit
	return stub.page, nil
}
