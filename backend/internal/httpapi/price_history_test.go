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
		}, prices: completeSevenDayPrices(time.Now(), 1), daily: completeThirtyDayPrices(time.Now(), 1)}
		response := analysisRequestTo(t, newAnalysisHTTPHandler(store), "/api/v1/analysis/market", analysisBody)
		if response.Code != http.StatusOK {
			t.Fatalf("status %d: %s", response.Code, response.Body.String())
		}
		var body priceHistoryBody
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Table.Rows) != 1 {
			t.Fatalf("missing 169-slot history: %+v", body.Table.Rows)
		}
		assertCompleteSevenDayHistory(t, body.window("price_history"), body.Table.Rows[0].prices())
		window := body.window("thirty_day_price_history")
		if window.From.Format(time.RFC3339) != "1999-12-01T00:00:00Z" || window.To.Format(time.RFC3339) != "1999-12-31T23:00:00Z" {
			t.Fatalf("wrong thirty-day window: %+v", window)
		}
		prices := body.Table.Rows[0].Cells["thirty_day_price_history"].Series
		if len(prices) != market.ThirtyDayPriceSlots || prices[0] == nil || *prices[0] != 1 || prices[30] == nil || *prices[30] != 31 || prices[31] == nil || *prices[31] != 199 {
			t.Fatalf("thirty-day history must hold 31 daily closes and the latest hourly close: %v", prices)
		}
	})
}

type priceHistoryBody struct {
	Table struct {
		Columns []struct {
			ID     string                     `json:"id"`
			Window *market.PriceHistoryWindow `json:"window"`
		} `json:"columns"`
		Rows []priceHistoryRow `json:"rows"`
	} `json:"table"`
}

func (body priceHistoryBody) window(id string) market.PriceHistoryWindow {
	for _, column := range body.Table.Columns {
		if column.ID == id && column.Window != nil {
			return *column.Window
		}
	}
	return market.PriceHistoryWindow{}
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
	prices []market.ClosePrice
	daily  []market.ClosePrice
	delay  time.Duration
}

// completeThirtyDayPrices closes the 31 days before the open one and the open
// day, which must be ignored, at their day number.
func completeThirtyDayPrices(at time.Time, instrumentID int64) []market.ClosePrice {
	today := market.IntervalDay.OpenTime(at)
	prices := make([]market.ClosePrice, 0, 32)
	for day := 1; day <= 32; day++ {
		prices = append(prices, market.ClosePrice{InstrumentID: instrumentID, OpenTime: today.AddDate(0, 0, day-32), Close: float64(day)})
	}
	return prices
}

func completeSevenDayPrices(at time.Time, instrumentID int64) []market.ClosePrice {
	prices := make([]market.ClosePrice, 0, 171)
	for hour := -170; hour <= 0; hour++ {
		prices = append(prices, market.ClosePrice{InstrumentID: instrumentID, OpenTime: at.Add(time.Duration(hour) * time.Hour), Close: float64(hour + 200)})
	}
	return prices
}

func shortSevenDayPrices(at time.Time, instrumentID int64) []market.ClosePrice {
	prices := make([]market.ClosePrice, 0, 73)
	for slot := 96; slot < market.SevenDayPriceSlots; slot++ {
		prices = append(prices, market.ClosePrice{InstrumentID: instrumentID, OpenTime: at.Add(time.Duration(slot-169) * time.Hour), Close: float64(slot)})
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

func (failingHistoryHTTPStore) ListClosePrices(context.Context, []int64, market.CandleInterval, time.Time, time.Time) ([]market.ClosePrice, error) {
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
		}, delay: 2 * time.Hour, prices: []market.ClosePrice{
			{InstrumentID: 1, OpenTime: time.Now().Add(-73 * time.Hour), Close: 10.00000001},
			{InstrumentID: 1, OpenTime: time.Now().Add(-71 * time.Hour), Close: 9.99999999},
			{InstrumentID: 3, OpenTime: time.Now().Add(-24 * time.Hour), Close: 3},
		}}
		response := analysisRequestTo(t, newAnalysisHTTPHandler(store), "/api/v1/analysis/market", analysisBody)
		if response.Code != http.StatusOK {
			t.Fatalf("status %d: %s", response.Code, response.Body.String())
		}
		var body struct {
			priceHistoryBody
			Matched int `json:"matched_count"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Matched != 3 || len(body.Table.Rows) != 3 {
			t.Fatalf("chart availability excluded results: %+v", body)
		}
		if window := body.window("price_history"); window.To.Format(time.RFC3339) != "1999-12-31T23:00:00Z" {
			t.Fatalf("window moved during analysis: %+v", window)
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

func (s priceHistoryHTTPStore) ListClosePrices(_ context.Context, ids []int64, interval market.CandleInterval, from, to time.Time) ([]market.ClosePrice, error) {
	prices := s.prices
	if interval == market.IntervalDay {
		prices = s.daily
	}
	var result []market.ClosePrice
	for _, price := range prices {
		for _, id := range ids {
			if price.InstrumentID == id && !price.OpenTime.Before(from) && !price.OpenTime.After(to) {
				result = append(result, price)
			}
		}
	}
	return result, nil
}
