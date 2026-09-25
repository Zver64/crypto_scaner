package indicator_test

import (
	"testing"
	"time"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
)

func TestEngineKeepsClosedHistoryIndependentOfLiveContext(t *testing.T) {
	registry, err := indicator.NewRegistry(talib.NewRSI())
	if err != nil {
		t.Fatal(err)
	}
	engine := indicator.NewEngine(registry)
	selections := []indicator.Selection{{Type: talib.RSIType, Parameters: indicator.Parameters{"period": 14}}}
	candles := make([]market.Candle, 200)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range candles {
		candles[i] = market.Candle{OpenTime: start.AddDate(0, 0, i), Close: float64(i + 1)}
	}
	closed, err := engine.Calculate(market.IntervalDay, candles, selections)
	if err != nil {
		t.Fatal(err)
	}
	live := append(append([]market.Candle(nil), candles...), market.Candle{OpenTime: start.AddDate(0, 0, 200), Close: 500})
	extended, err := engine.Calculate(market.IntervalDay, live, selections)
	if err != nil {
		t.Fatal(err)
	}
	points := closed[0].Series[0].Points
	if len(points) != 186 {
		t.Fatalf("closed points = %d, want 186", len(points))
	}
	if len(extended[0].Series[0].Points) != len(points)+1 {
		t.Fatal("live point missing")
	}
	for i, point := range points {
		if point != extended[0].Series[0].Points[i] {
			t.Fatalf("closed point %d changed with live price", i)
		}
	}
	if len(candles) != 200 || candles[199].Close != 200 {
		t.Fatal("live context mutated closed history")
	}
}

func TestEngineDoesNotWarmAcrossGaps(t *testing.T) {
	registry, _ := indicator.NewRegistry(talib.NewRSI())
	engine := indicator.NewEngine(registry)
	candles := make([]market.Candle, 16)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range candles {
		offset := i
		if i >= 8 {
			offset++
		}
		candles[i] = market.Candle{OpenTime: start.AddDate(0, 0, offset), Close: float64(i + 1)}
	}
	result, err := engine.Calculate(market.IntervalDay, candles, []indicator.Selection{{Type: talib.RSIType, Parameters: indicator.Parameters{"period": 14}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result[0].Series[0].Points) != 0 {
		t.Fatal("RSI warmed across missing day")
	}
}
