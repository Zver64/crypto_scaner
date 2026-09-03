package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"crypto-scanner/internal/market"
)

func TestMarketAPIIncludesFixedSevenDayWindowAndClosedPrices(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// synctest starts at 2000-01-01 00:00 UTC, crossing a day/year boundary.
		store := priceHistoryHTTPStore{httpStore: httpStore{
			instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT"}},
			candles:     map[int64][]market.Candle{1: {httpCandle(time.Now().Add(-24*time.Hour), 2)}},
		}}
		for hour := -170; hour <= 0; hour++ {
			store.prices = append(store.prices, market.HourlyPrice{InstrumentID: 1, OpenTime: time.Now().Add(time.Duration(hour) * time.Hour), Close: float64(hour + 200)})
		}
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
		if body.Window.From.Format(time.RFC3339) != "1999-12-24T23:00:00Z" || body.Window.To.Format(time.RFC3339) != "1999-12-31T23:00:00Z" {
			t.Fatalf("wrong window: %+v", body.Window)
		}
		if len(body.Items) != 1 || len(body.Items[0].Prices) != 169 {
			t.Fatalf("missing 169-slot history: %+v", body.Items)
		}
		prices := body.Items[0].Prices
		if prices[0] == nil || *prices[0] != 31 || prices[168] == nil || *prices[168] != 199 {
			t.Fatalf("wrong closed endpoints: %v / %v", prices[0], prices[168])
		}
		for i, price := range prices {
			if price == nil {
				t.Fatalf("missing slot %d", i)
			}
		}
	})
}

type priceHistoryHTTPStore struct {
	httpStore
	prices []market.HourlyPrice
	delay  time.Duration
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
