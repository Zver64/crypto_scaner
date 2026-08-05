package binance_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"crypto-scanner/internal/exchange/binance"
	"crypto-scanner/internal/market"
	marketsync "crypto-scanner/internal/market/sync"

	"golang.org/x/time/rate"
)

var _ marketsync.Exchange = (*binance.Exchange)(nil)

func TestExchangeListInstrumentsMapsCompleteSpotUSDTSnapshot(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/v3/exchangeInfo" {
			t.Errorf("path = %q, want /api/v3/exchangeInfo", request.URL.Path)
		}
		if got := request.URL.Query().Get("permissions"); got != "SPOT" {
			t.Errorf("permissions = %q, want SPOT", got)
		}
		body := `{
			"timezone":"UTC",
			"symbols":[
				{"symbol":"BTCUSDT","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","permissions":["SPOT"],"isSpotTradingAllowed":true},
				{"symbol":"ETHUSDT","status":"BREAK","baseAsset":"ETH","quoteAsset":"USDT","permissionSets":[["SPOT","MARGIN"]],"isSpotTradingAllowed":false},
				{"symbol":"BTCEUR","status":"TRADING","baseAsset":"BTC","quoteAsset":"EUR","permissions":["SPOT"],"isSpotTradingAllowed":true},
				{"symbol":"FUTUSDT","status":"TRADING","baseAsset":"FUT","quoteAsset":"USDT","permissions":["MARGIN"],"isSpotTradingAllowed":false}
			]
		}`
		return jsonResponse(body), nil
	})}

	exchange := binance.NewWithHTTPClient("https://fixture.invalid", httpClient)
	got, err := exchange.ListInstruments(context.Background())
	if err != nil {
		t.Fatalf("ListInstruments() error = %v", err)
	}
	want := []market.Instrument{
		{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Status: "TRADING", Active: true},
		{Symbol: "ETHUSDT", BaseAsset: "ETH", QuoteAsset: "USDT", Status: "BREAK", Active: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListInstruments() = %#v, want %#v", got, want)
	}
}

func TestExchangeListInstrumentsRejectsIncompleteSnapshot(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"timezone":"UTC","symbols":[]}`), nil
	})}

	if _, err := binance.NewWithHTTPClient("https://fixture.invalid", httpClient).ListInstruments(context.Background()); err == nil {
		t.Fatal("ListInstruments() accepted an empty discovery snapshot")
	}
}

func TestExchangeListInstrumentsRejectsSnapshotContainingMalformedEntry(t *testing.T) {
	tests := map[string]string{
		"missing required field before eligibility filtering": `{
			"symbols":[
				{"symbol":"BTCUSDT","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","permissions":["SPOT"]},
				{"symbol":"BTCEUR","status":"TRADING","baseAsset":"","quoteAsset":"EUR","permissions":["SPOT"]}
			]
		}`,
		"ambiguous USDT permission metadata": `{
			"symbols":[
				{"symbol":"BTCUSDT","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","permissions":["SPOT"]},
				{"symbol":"MAYBEUSDT","status":"BREAK","baseAsset":"MAYBE","quoteAsset":"USDT"}
			]
		}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(body), nil
			})}
			if _, err := binance.NewWithHTTPClient("https://fixture.invalid", httpClient).ListInstruments(context.Background()); err == nil {
				t.Fatal("ListInstruments() accepted a partially malformed discovery snapshot")
			}
		})
	}
}

func TestExchangeListInstrumentsMapsEveryKnownInactiveStatus(t *testing.T) {
	body := `{
		"symbols":[
			{"symbol":"PREUSDT","status":"PRE_TRADING","baseAsset":"PRE","quoteAsset":"USDT","permissions":["SPOT"]},
			{"symbol":"POSTUSDT","status":"POST_TRADING","baseAsset":"POST","quoteAsset":"USDT","permissions":["SPOT"]},
			{"symbol":"ENDUSDT","status":"END_OF_DAY","baseAsset":"END","quoteAsset":"USDT","permissions":["SPOT"]},
			{"symbol":"HALTUSDT","status":"HALT","baseAsset":"HALT","quoteAsset":"USDT","permissions":["SPOT"]},
			{"symbol":"AUCTIONUSDT","status":"AUCTION_MATCH","baseAsset":"AUCTION","quoteAsset":"USDT","permissions":["SPOT"]},
			{"symbol":"BREAKUSDT","status":"BREAK","baseAsset":"BREAK","quoteAsset":"USDT","permissions":["SPOT"]}
		]
	}`
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(body), nil
	})}

	got, err := binance.NewWithHTTPClient("https://fixture.invalid", httpClient).ListInstruments(context.Background())
	if err != nil {
		t.Fatalf("ListInstruments() error = %v", err)
	}
	wantStatuses := []string{"PRE_TRADING", "POST_TRADING", "END_OF_DAY", "HALT", "AUCTION_MATCH", "BREAK"}
	if len(got) != len(wantStatuses) {
		t.Fatalf("ListInstruments() = %#v, want %d known inactive instruments", got, len(wantStatuses))
	}
	for index, status := range wantStatuses {
		if got[index].Status != status || got[index].Active {
			t.Fatalf("instrument %d = %#v, want inactive status %q", index, got[index], status)
		}
	}
}

func TestExchangeListInstrumentsRejectsUnknownStatusWithoutReturningPartialSnapshot(t *testing.T) {
	body := `{
		"symbols":[
			{"symbol":"BTCUSDT","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","permissions":["SPOT"]},
			{"symbol":"TYPOUSDT","status":"TRAIDNG","baseAsset":"TYPO","quoteAsset":"USDT","permissions":["SPOT"]}
		]
	}`
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(body), nil
	})}

	items, err := binance.NewWithHTTPClient("https://fixture.invalid", httpClient).ListInstruments(context.Background())
	if err == nil {
		t.Fatalf("ListInstruments() = %#v, want unknown-status error and no snapshot", items)
	}
	if items != nil {
		t.Fatalf("ListInstruments() returned partial snapshot %#v after unknown status", items)
	}
}

func TestExchangeListClosedCandlesRequestsLatestDailyHistoryAndMapsValues(t *testing.T) {
	cutoff := time.Date(2026, time.August, 5, 0, 0, 30, 0, time.UTC)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/v3/klines" {
			t.Errorf("path = %q, want /api/v3/klines", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("symbol") != "BTCUSDT" || query.Get("interval") != "1d" || query.Get("limit") != "30" {
			t.Errorf("query = %q, want BTCUSDT daily limit 30", query.Encode())
		}
		if got := query.Get("endTime"); got != "1785887999999" {
			t.Errorf("endTime = %q, want last millisecond before current UTC day", got)
		}
		return jsonResponse(`[[1785801600000,"114325.12345678","115000.87654321","112900.00000001","113750.99999999","1234.56789012",1785887999999,"140000000.12345678",98765,"600.1","68000000.2","0"]]`), nil
	})}

	got, err := binance.NewWithHTTPClient("https://fixture.invalid", httpClient).ListClosedCandles(context.Background(), market.CandleRequest{
		Symbol: "BTCUSDT", Interval: "1d", Limit: 30, ClosedBefore: cutoff,
	})
	if err != nil {
		t.Fatalf("ListClosedCandles() error = %v", err)
	}
	want := []market.Candle{{
		Interval: "1d", OpenTime: time.Date(2026, time.August, 4, 0, 0, 0, 0, time.UTC),
		CloseTime: time.Date(2026, time.August, 4, 23, 59, 59, 999000000, time.UTC),
		Open:      114325.12345678, High: 115000.87654321, Low: 112900.00000001, Close: 113750.99999999,
		Volume: 1234.56789012, QuoteAssetVolume: 140000000.12345678, TradeCount: 98765,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListClosedCandles() = %#v, want %#v", got, want)
	}
}

func TestExchangeListClosedCandlesExcludesFormingCandleAndAcceptsShortHistory(t *testing.T) {
	cutoff := time.Date(2026, time.August, 5, 0, 0, 30, 0, time.UTC)
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`[
			[1785801600000,"1","2","0.5","1.5","10",1785887999999,"15",3,"0","0","0"],
			[1785888000000,"1.5","2.5","1","2","20",1785974399999,"40",4,"0","0","0"]
		]`), nil
	})}

	got, err := binance.NewWithHTTPClient("https://fixture.invalid", httpClient).ListClosedCandles(context.Background(), market.CandleRequest{
		Symbol: "NEWUSDT", Interval: "1d", Limit: 30, ClosedBefore: cutoff,
	})
	if err != nil {
		t.Fatalf("ListClosedCandles() error = %v", err)
	}
	if len(got) != 1 || got[0].OpenTime != time.Date(2026, time.August, 4, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("ListClosedCandles() = %#v, want only the available closed candle", got)
	}
}

func TestExchangeListClosedCandlesStartsAfterLatestStoredOpenTime(t *testing.T) {
	latest := time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.URL.Query().Get("startTime"); got != "1785628800001" {
			t.Errorf("startTime = %q, want one millisecond after latest open time", got)
		}
		return jsonResponse(`[]`), nil
	})}

	_, err := binance.NewWithHTTPClient("https://fixture.invalid", httpClient).ListClosedCandles(context.Background(), market.CandleRequest{
		Symbol: "BTCUSDT", Interval: "1d", Limit: 1000,
		ClosedBefore: time.Date(2026, time.August, 5, 0, 0, 30, 0, time.UTC), AfterOpenTime: &latest,
	})
	if err != nil {
		t.Fatalf("ListClosedCandles() error = %v", err)
	}
}

func TestExchangeRetriesServerFailuresButNotPermanentClientFailures(t *testing.T) {
	t.Run("server failure", func(t *testing.T) {
		calls := 0
		exchange := binance.NewWithOptions(binance.Options{
			BaseURL: "https://fixture.invalid", RetryAttempts: 3, RetryBaseDelay: time.Millisecond,
			Limiter: rate.NewLimiter(rate.Inf, 1), HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				if calls < 3 {
					return statusResponse(http.StatusInternalServerError, `{"code":-1,"msg":"temporary"}`), nil
				}
				return jsonResponse(`{"symbols":[{"symbol":"BTCUSDT","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","permissions":["SPOT"]}]}`), nil
			})},
		})
		if _, err := exchange.ListInstruments(context.Background()); err != nil {
			t.Fatalf("ListInstruments() error = %v", err)
		}
		if calls != 3 {
			t.Fatalf("calls = %d, want 3", calls)
		}
		if exchange.RetryCount() != 2 {
			t.Fatalf("retry count = %d, want 2", exchange.RetryCount())
		}
	})

	t.Run("permanent client failure", func(t *testing.T) {
		calls := 0
		exchange := binance.NewWithOptions(binance.Options{
			BaseURL: "https://fixture.invalid", RetryAttempts: 5, RetryBaseDelay: time.Millisecond,
			Limiter: rate.NewLimiter(rate.Inf, 1), HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return statusResponse(http.StatusBadRequest, `{"code":-1100,"msg":"bad request"}`), nil
			})},
		})
		if _, err := exchange.ListInstruments(context.Background()); err == nil {
			t.Fatal("ListInstruments() accepted permanent client failure")
		}
		if calls != 1 {
			t.Fatalf("calls = %d, want no retry", calls)
		}
	})
}

func TestExchangeUsesDiscoveryRequestWeightMetadata(t *testing.T) {
	limiter := rate.NewLimiter(rate.Inf, 4)
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"rateLimits":[{"rateLimitType":"REQUEST_WEIGHT","interval":"MINUTE","limit":1200}],"symbols":[{"symbol":"BTCUSDT","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","permissions":["SPOT"]}]}`), nil
	})}
	exchange := binance.NewWithOptions(binance.Options{BaseURL: "https://fixture.invalid", HTTPClient: httpClient, Limiter: limiter})

	if _, err := exchange.ListInstruments(context.Background()); err != nil {
		t.Fatalf("ListInstruments() error = %v", err)
	}
	if got := limiter.Limit(); got != rate.Limit(18) {
		t.Fatalf("shared limiter rate = %v, want 18 requests/second with headroom", got)
	}
}

func TestExchangeRetryBackoffHonorsCancellation(t *testing.T) {
	called := make(chan struct{}, 1)
	exchange := binance.NewWithOptions(binance.Options{
		BaseURL: "https://fixture.invalid", RetryAttempts: 5, RetryBaseDelay: time.Hour,
		Limiter: rate.NewLimiter(rate.Inf, 1), HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			called <- struct{}{}
			return nil, errors.New("temporary transport failure")
		})},
	})
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := exchange.ListInstruments(ctx)
		result <- err
	}()
	<-called
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ListInstruments() error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("retry backoff ignored cancellation")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func jsonResponse(body string) *http.Response {
	return statusResponse(http.StatusOK, body)
}

func statusResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
