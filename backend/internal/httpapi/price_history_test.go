package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/market"
)

func TestMarketAPIIncludesFixedSevenDayWindowAndClosedPrices(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// synctest starts at 2000-01-01 00:00 UTC, crossing a day/year boundary.
		store := priceHistoryHTTPStore{httpStore: httpStore{
			instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}},
			candles:     map[int64][]market.Candle{1: httpCandles(time.Now().Add(-24*time.Hour), 2, 24*time.Hour, 2)},
		}, prices: completeSevenDayPrices(time.Now(), 1)}
		response := analysisRequestTo(t, newAnalysisHTTPHandler(store), "/api/v1/analysis/market", analysisBody)
		if response.Code != http.StatusOK {
			t.Fatalf("status %d: %s", response.Code, response.Body.String())
		}
		var body struct {
			Window market.PriceHistoryWindow `json:"price_history_window"`
			Table  struct {
				Rows []priceHistoryRow `json:"rows"`
			} `json:"table"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Table.Rows) != 1 {
			t.Fatalf("missing 169-slot history: %+v", body.Table.Rows)
		}
		assertCompleteSevenDayHistory(t, body.Window, body.Table.Rows[0].prices())
	})
}

type priceHistoryRow struct {
	Symbol string `json:"symbol"`
	Cells  map[string]struct {
		Series []*float64 `json:"series"`
	} `json:"cells"`
}

func (row priceHistoryRow) prices() []*float64 { return row.Cells["price_history"].Series }

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

func (failingHistoryHTTPStore) ListHourlyPrices(context.Context, []int64, time.Time, time.Time) ([]market.HourlyPrice, error) {
	return nil, errors.New("database unavailable")
}

func (s priceHistoryHTTPStore) ListActiveInstruments(ctx context.Context) ([]market.Instrument, error) {
	time.Sleep(s.delay)
	return s.httpStore.ListActiveInstruments(ctx)
}
func (s priceHistoryHTTPStore) SelectActiveInstruments(ctx context.Context, selection analysis.Selection) ([]market.Instrument, error) {
	time.Sleep(s.delay)
	return s.httpStore.SelectActiveInstruments(ctx, selection)
}

func TestMarketAPIKeepsMissingHistoryAndFreezesWindowBeforeSlowAnalysis(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := priceHistoryHTTPStore{httpStore: httpStore{
			instruments: []market.Instrument{{ID: 1, Symbol: "PARTIAL"}, {ID: 2, Symbol: "EMPTY"}, {ID: 3, Symbol: "SINGLE"}},
			candles: map[int64][]market.Candle{
				1: httpCandles(time.Now().Add(-24*time.Hour), 2, 24*time.Hour, 2),
				2: httpCandles(time.Now().Add(-24*time.Hour), 2, 24*time.Hour, 2),
				3: httpCandles(time.Now().Add(-24*time.Hour), 2, 24*time.Hour, 2),
			},
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
			Table   struct {
				Rows []priceHistoryRow `json:"rows"`
			} `json:"table"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Matched != 3 || len(body.Table.Rows) != 3 {
			t.Fatalf("chart availability excluded results: %+v", body)
		}
		if body.Window.To.Format(time.RFC3339) != "1999-12-31T23:00:00Z" {
			t.Fatalf("window moved during analysis: %+v", body.Window)
		}
		for _, item := range body.Table.Rows {
			if len(item.prices()) != 169 {
				t.Fatalf("%s has %d slots", item.Symbol, len(item.prices()))
			}
			for i, price := range item.prices() {
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
