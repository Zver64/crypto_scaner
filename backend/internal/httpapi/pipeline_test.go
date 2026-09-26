package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	marketcapcriterion "crypto-scanner/internal/analysis/criteria/marketcap"
	"crypto-scanner/internal/market"
)

const pipelineBody = `{"criteria":[{"key":"daily_volatility","name":"volatility","label":"Daily Volatility","parameters":{"unit":"days","period":30,"percentile":80,"minimum_range_percent":5}},{"key":"hourly_volatility","name":"volatility","label":"Hourly Volatility","parameters":{"unit":"hours","period":60,"percentile":80,"minimum_range_percent":2}},{"key":"market_cap","name":"market_cap","label":"Market Cap","parameters":{"min_market_cap_usd":500000000}}]}`

func TestMarketAPIExecutesUnifiedPipeline(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	cap := 500_000_000.0
	store := httpStore{
		instruments: []market.Instrument{{ID: 1, Symbol: "BTCUSDT", BaseAsset: "BTC", MarketCapUSD: &cap}, {ID: 2, Symbol: "DROPUSDT", BaseAsset: "DROP", MarketCapUSD: &cap}},
		candlesByInterval: map[int64]map[string][]market.Candle{
			1: {"1d": httpCandles(start, 30, 24*time.Hour, 6), "1h": httpCandles(start, 60, time.Hour, 3)},
			2: {"1d": httpCandles(start, 30, 24*time.Hour, 4)},
		},
	}
	response := analysisRequestTo(t, newAnalysisHTTPHandler(store, marketcapcriterion.New()), "/api/v1/analysis/market", pipelineBody)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body marketAnalysisResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.MatchedCount != 1 || body.AnalyzedCount != 2 || body.InsufficientDataCount != 0 || len(body.Items) != 1 || body.Items[0].Symbol != "BTCUSDT" {
		t.Fatalf("body=%+v", body)
	}
	evaluations := body.Items[0].Evaluations
	if len(evaluations) != 3 {
		t.Fatalf("evaluations=%+v", evaluations)
	}
	for i, want := range []struct {
		key, name, label, metric string
		value                    float64
		candles                  int
	}{
		{"daily_volatility", "volatility", "Daily Volatility", "range_percent", 6, 30},
		{"hourly_volatility", "volatility", "Hourly Volatility", "range_percent", 3, 60},
		{"market_cap", "market_cap", "Market Cap", "market_cap_usd", 500_000_000, 0},
	} {
		got := evaluations[i]
		if got.Key != want.key || got.Name != want.name || got.Label != want.label || !got.Matched || got.Metrics[want.metric] != want.value || got.CandleCount != want.candles {
			t.Fatalf("evaluation=%+v want=%+v", got, want)
		}
	}
}

func TestMarketAPIValidatesEachVolatilityInstance(t *testing.T) {
	for _, period := range []string{`"period":30`, `"period":60`} {
		t.Run(period, func(t *testing.T) {
			body := strings.Replace(pipelineBody, period, `"period":0`, 1)
			response := analysisRequestTo(t, newAnalysisHTTPHandler(httpStore{}, marketcapcriterion.New()), "/api/v1/analysis/market", body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
