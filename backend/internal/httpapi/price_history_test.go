package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"crypto-scanner/internal/market"
)

func TestInstrumentAPIIncludesUpToThirtyDaysOfClosedHourlyCandles(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// synctest starts at 2000-01-01 00:00 UTC, crossing a day/year boundary.
		store := priceHistoryHTTPStore{httpStore: httpStore{
			instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}},
			candles:     map[int64][]market.Candle{1: {httpCandle(time.Now().Add(-24*time.Hour), 2)}},
		}, delay: 2 * time.Hour, prices: completeSevenDayPrices(time.Now(), 1)}

		response := analysisRequestTo(t, newAnalysisHTTPHandler(store), "/api/v1/analysis/instruments/BTCUSDT", analysisBody)
		if response.Code != http.StatusOK {
			t.Fatalf("status %d: %s", response.Code, response.Body.String())
		}
		var body struct {
			Symbol      string                    `json:"symbol"`
			Evaluations []evaluationResponse      `json:"evaluations"`
			Window      market.PriceHistoryWindow `json:"price_history_window"`
			Candles     []*market.HourlyCandle    `json:"candle_history"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Symbol != "BTCUSDT" || len(body.Evaluations) != 1 {
			t.Fatalf("missing analysis response values: %+v", body)
		}
		if body.Window.From.Format(time.RFC3339) != "1999-12-01T23:00:00Z" || body.Window.To.Format(time.RFC3339) != "1999-12-31T23:00:00Z" {
			t.Fatalf("wrong thirty-day window: %+v", body.Window)
		}
		if len(body.Candles) != market.ThirtyDayPriceSlots || body.Candles[550] != nil || body.Candles[551] == nil || body.Candles[551].Close != 30 || body.Candles[720] == nil || body.Candles[720].Close != 199 {
			t.Fatalf("available month candles were not returned in fixed slots: %v / %v (%d)", body.Candles[551], body.Candles[720], len(body.Candles))
		}
	})
}

func TestInstrumentAPIKeepsGappedIsolatedStaleAndEmptyHistory(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := priceHistoryHTTPStore{httpStore: httpStore{
			instruments: []market.Instrument{{ID: 1, Symbol: "PARTIAL"}, {ID: 2, Symbol: "EMPTY"}, {ID: 3, Symbol: "SINGLE"}, {ID: 4, Symbol: "STALE"}, {ID: 5, Symbol: "SHORT"}},
			candles: map[int64][]market.Candle{
				1: {httpCandle(time.Now(), 2)}, 2: {httpCandle(time.Now(), 2)}, 3: {httpCandle(time.Now(), 2)}, 4: {httpCandle(time.Now(), 2)}, 5: {httpCandle(time.Now(), 2)},
			},
		}, prices: []market.HourlyPrice{
			{InstrumentID: 1, OpenTime: time.Now().Add(-73 * time.Hour), Close: 10.00000001},
			{InstrumentID: 1, OpenTime: time.Now().Add(-71 * time.Hour), Close: 9.99999999},
			{InstrumentID: 3, OpenTime: time.Now().Add(-24 * time.Hour), Close: 3},
			{InstrumentID: 4, OpenTime: time.Now().Add(-200 * time.Hour), Close: 4},
		}}
		store.prices = append(store.prices, shortSevenDayPrices(time.Now(), 5)...)

		for _, test := range []struct {
			symbol string
			slots  map[int]float64
			short  bool
		}{
			{symbol: "PARTIAL", slots: map[int]float64{648: 10.00000001, 650: 9.99999999}},
			{symbol: "EMPTY", slots: map[int]float64{}},
			{symbol: "SINGLE", slots: map[int]float64{697: 3}},
			{symbol: "STALE", slots: map[int]float64{521: 4}},
			{symbol: "SHORT", short: true},
		} {
			response := analysisRequestTo(t, newAnalysisHTTPHandler(store), "/api/v1/analysis/instruments/"+test.symbol, analysisBody)
			if response.Code != http.StatusOK {
				t.Fatalf("%s status %d: %s", test.symbol, response.Code, response.Body.String())
			}
			var body struct {
				Candles []*market.HourlyCandle `json:"candle_history"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Candles) != market.ThirtyDayPriceSlots {
				t.Fatalf("%s history slots = %d", test.symbol, len(body.Candles))
			}
			for slot, candle := range body.Candles {
				want, exists := test.slots[slot]
				if test.short && slot >= 648 {
					want, exists = float64(slot-552), true
				}
				if exists && (candle == nil || candle.Close != want) {
					t.Fatalf("%s slot %d = %v, want close %v", test.symbol, slot, candle, want)
				}
				if !exists && candle != nil {
					t.Fatalf("%s slot %d = %v, want missing", test.symbol, slot, candle)
				}
			}
		}
	})
}

func TestInstrumentAPIKeepsHistoryStoreFailuresCanonical(t *testing.T) {
	store := failingHistoryHTTPStore{httpStore: httpStore{
		instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}},
		candles:     map[int64][]market.Candle{1: {httpCandle(time.Now(), 2)}},
	}}
	response := analysisRequestTo(t, newAnalysisHTTPHandler(store), "/api/v1/analysis/instruments/BTCUSDT", analysisBody)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "internal_error" {
		t.Fatalf("error = %+v", body.Error)
	}
}

func TestMarketAPIIncludesFixedSevenDayWindowAndClosedPrices(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// synctest starts at 2000-01-01 00:00 UTC, crossing a day/year boundary.
		store := priceHistoryHTTPStore{httpStore: httpStore{
			instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}},
			candles:     map[int64][]market.Candle{1: {httpCandle(time.Now().Add(-24*time.Hour), 2)}},
		}, prices: completeSevenDayPrices(time.Now(), 1)}
		response := analysisRequestTo(t, newAnalysisHTTPHandler(store), "/api/v1/analysis/market", analysisBody)
		if response.Code != http.StatusOK {
			t.Fatalf("status %d: %s", response.Code, response.Body.String())
		}
		var body struct {
			Window market.PriceHistoryWindow `json:"price_history_window"`
			Items  []struct {
				Prices []*float64 `json:"price_history"`
			} `json:"items"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Items) != 1 {
			t.Fatalf("missing 169-slot history: %+v", body.Items)
		}
		assertCompleteSevenDayHistory(t, body.Window, body.Items[0].Prices)
	})
}

type priceHistoryHTTPStore struct {
	httpStore
	prices []market.HourlyPrice
	delay  time.Duration
}

func completeSevenDayPrices(at time.Time, instrumentID int64) []market.HourlyPrice {
	prices := make([]market.HourlyPrice, 0, 171)
	for hour := -170; hour <= 0; hour++ {
		prices = append(prices, market.HourlyPrice{InstrumentID: instrumentID, OpenTime: at.Add(time.Duration(hour) * time.Hour), Close: float64(hour + 200)})
	}
	return prices
}

func shortSevenDayPrices(at time.Time, instrumentID int64) []market.HourlyPrice {
	prices := make([]market.HourlyPrice, 0, 73)
	for slot := 96; slot < market.SevenDayPriceSlots; slot++ {
		prices = append(prices, market.HourlyPrice{InstrumentID: instrumentID, OpenTime: at.Add(time.Duration(slot-169) * time.Hour), Close: float64(slot)})
	}
	return prices
}

func assertCompleteSevenDayHistory(t *testing.T, window market.PriceHistoryWindow, prices []*float64) {
	t.Helper()
	if window.From.Format(time.RFC3339) != "1999-12-24T23:00:00Z" || window.To.Format(time.RFC3339) != "1999-12-31T23:00:00Z" {
		t.Fatalf("wrong frozen window: %+v", window)
	}
	if len(prices) != market.SevenDayPriceSlots {
		t.Fatalf("missing 169-slot history: %d", len(prices))
	}
	if prices[0] == nil || *prices[0] != 31 || prices[168] == nil || *prices[168] != 199 {
		t.Fatalf("open hour was included or closed endpoints were wrong: %v / %v", prices[0], prices[168])
	}
}

type failingHistoryHTTPStore struct{ httpStore }

func (failingHistoryHTTPStore) ListHourlyCandles(context.Context, int64, time.Time, time.Time) ([]market.HourlyCandle, error) {
	return nil, errors.New("database unavailable")
}

func (failingHistoryHTTPStore) ListHourlyPrices(context.Context, []int64, time.Time, time.Time) ([]market.HourlyPrice, error) {
	return nil, errors.New("database unavailable")
}

func (s priceHistoryHTTPStore) ListActiveInstruments(ctx context.Context) ([]market.Instrument, error) {
	time.Sleep(s.delay)
	return s.httpStore.ListActiveInstruments(ctx)
}

func TestMarketAPIKeepsMissingHistoryAndFreezesWindowBeforeSlowAnalysis(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := priceHistoryHTTPStore{httpStore: httpStore{
			instruments: []market.Instrument{{ID: 1, Symbol: "PARTIAL"}, {ID: 2, Symbol: "EMPTY"}, {ID: 3, Symbol: "SINGLE"}},
			candles:     map[int64][]market.Candle{1: {httpCandle(time.Now(), 2)}, 2: {httpCandle(time.Now(), 2)}, 3: {httpCandle(time.Now(), 2)}},
		}, delay: 2 * time.Hour, prices: []market.HourlyPrice{
			{InstrumentID: 1, OpenTime: time.Now().Add(-73 * time.Hour), Close: 10.00000001},
			{InstrumentID: 1, OpenTime: time.Now().Add(-71 * time.Hour), Close: 9.99999999},
			{InstrumentID: 3, OpenTime: time.Now().Add(-24 * time.Hour), Close: 3},
		}}
		response := analysisRequestTo(t, newAnalysisHTTPHandler(store), "/api/v1/analysis/market", analysisBody)
		if response.Code != http.StatusOK {
			t.Fatalf("status %d: %s", response.Code, response.Body.String())
		}
		var body struct {
			Window  market.PriceHistoryWindow `json:"price_history_window"`
			Matched int                       `json:"matched_count"`
			Items   []struct {
				Symbol string     `json:"symbol"`
				Prices []*float64 `json:"price_history"`
			} `json:"items"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Matched != 3 || len(body.Items) != 3 {
			t.Fatalf("chart availability excluded results: %+v", body)
		}
		if body.Window.To.Format(time.RFC3339) != "1999-12-31T23:00:00Z" {
			t.Fatalf("window moved during analysis: %+v", body.Window)
		}
		for _, item := range body.Items {
			if len(item.Prices) != 169 {
				t.Fatalf("%s has %d slots", item.Symbol, len(item.Prices))
			}
			for i, price := range item.Prices {
				var want *float64
				if item.Symbol == "PARTIAL" && i == 96 {
					value := 10.00000001
					want = &value
				}
				if item.Symbol == "PARTIAL" && i == 98 {
					value := 9.99999999
					want = &value
				}
				if item.Symbol == "SINGLE" && i == 145 {
					value := 3.0
					want = &value
				}
				if want == nil && price != nil || want != nil && (price == nil || *price != *want) {
					t.Fatalf("%s slot %d: got %v, want %v", item.Symbol, i, price, want)
				}
			}
		}
	})
}

func (s priceHistoryHTTPStore) ListHourlyCandles(_ context.Context, id int64, from, to time.Time) ([]market.HourlyCandle, error) {
	var result []market.HourlyCandle
	for _, price := range s.prices {
		if price.InstrumentID == id && !price.OpenTime.Before(from) && !price.OpenTime.After(to) {
			result = append(result, market.HourlyCandle{
				InstrumentID: id, OpenTime: price.OpenTime, Open: price.Close - 0.5,
				High: price.Close + 1, Low: price.Close - 1, Close: price.Close,
			})
		}
	}
	return result, nil
}

func (s priceHistoryHTTPStore) ListHourlyPrices(_ context.Context, ids []int64, from, to time.Time) ([]market.HourlyPrice, error) {
	var result []market.HourlyPrice
	for _, price := range s.prices {
		for _, id := range ids {
			if price.InstrumentID == id && !price.OpenTime.Before(from) && !price.OpenTime.After(to) {
				result = append(result, price)
			}
		}
	}
	return result, nil
}
