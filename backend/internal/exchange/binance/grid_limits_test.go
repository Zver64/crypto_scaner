package binance_test

import (
	"context"
	"net/http"
	"testing"

	"crypto-scanner/internal/exchange/binance"
	"crypto-scanner/internal/market/gridlimits"
)

func TestExchangeSpotFiltersParsesPriceSizeAndNotionalFilters(t *testing.T) {
	const filters = `{"filterType":"PRICE_FILTER","minPrice":"0.01","maxPrice":"1000000","tickSize":"0.01"},
		{"filterType":"LOT_SIZE","minQty":"0.00001","maxQty":"9000","stepSize":"0.00001"},
		{"filterType":"MARKET_LOT_SIZE","minQty":"0","maxQty":"80","stepSize":"0"},
		{"filterType":"PERCENT_PRICE_BY_SIDE","bidMultiplierUp":"5","bidMultiplierDown":"0.2","askMultiplierUp":"5","askMultiplierDown":"0.2","avgPriceMins":5}`
	tests := []struct {
		name     string
		notional string
		want     float64
	}{
		{"NOTIONAL wins over MIN_NOTIONAL", `,{"filterType":"MIN_NOTIONAL","minNotional":"10"},{"filterType":"NOTIONAL","minNotional":"5","maxNotional":"9000000"}`, 5},
		{"MIN_NOTIONAL without NOTIONAL", `,{"filterType":"MIN_NOTIONAL","minNotional":"10"}`, 10},
		{"neither", ``, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.Path != "/api/v3/exchangeInfo" || request.URL.Query().Get("symbol") != "BTCUSDT" {
					t.Errorf("request = %s", request.URL)
				}
				return jsonResponse(`{"symbols":[{"symbol":"BTCUSDT","filters":[` + filters + test.notional + `]}]}`), nil
			})}
			got, err := binance.New(discardLogger, binance.Options{BaseURL: "https://fixture.invalid", HTTPClient: httpClient}).SpotFilters(context.Background(), "btcusdt")
			if err != nil {
				t.Fatal(err)
			}
			want := gridlimits.SpotFilters{
				MinPrice: 0.01, MaxPrice: 1000000, TickSize: 0.01, BidMultiplierDown: 0.2, AskMultiplierUp: 5,
				MinQty: 0.00001, StepSize: 0.00001, MinNotional: test.want,
			}
			if got != want {
				t.Fatalf("filters = %+v, want %+v", got, want)
			}
		})
	}
}
