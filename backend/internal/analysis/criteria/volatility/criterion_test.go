package volatility_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/analysis/criteria/volatility"
	"crypto-scanner/internal/market"
)

func TestFactoryExposesVolatilityCriterionType(t *testing.T) {
	factory := volatility.New()
	if factory.Name() != "volatility" {
		t.Fatalf("factory name = %q", factory.Name())
	}
	criterion, err := factory.Build(map[string]any{"unit": "days", "period": float64(1), "percentile": float64(50), "minimum_range_percent": float64(0)})
	if err != nil {
		t.Fatal(err)
	}
	if criterion.Name() != "volatility" {
		t.Fatalf("criterion name = %q", criterion.Name())
	}
}

func TestPercentileRequiresConfiguredShareToMeetThreshold(t *testing.T) {
	c, err := volatility.New().Build(map[string]any{"unit": "days", "period": float64(4), "percentile": float64(75), "minimum_range_percent": float64(2)})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	data := []market.Candle{candle(start, 8), candle(start.AddDate(0, 0, 1), 1), candle(start.AddDate(0, 0, 2), 4), candle(start.AddDate(0, 0, 3), 2)}
	result, err := c.Evaluate(context.Background(), analysis.Input{Candles: map[analysis.Unit][]market.Candle{analysis.UnitDays: data}})
	if err != nil || result.Metrics["range_percent"] != 2 || !result.Matched {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestPercentileDoesNotDependOnRangeOrderWithinWindow(t *testing.T) {
	c, err := volatility.New().Build(map[string]any{"unit": "days", "period": float64(4), "percentile": float64(75), "minimum_range_percent": float64(0)})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	chronological := func(ranges ...float64) []market.Candle {
		candles := make([]market.Candle, len(ranges))
		for i, r := range ranges {
			candles[i] = candle(start.AddDate(0, 0, i), r)
		}
		return candles
	}
	// Both inputs are chronological and fully inside the window; only the
	// order of the ranges across days differs.
	candles := chronological(8, 1, 4, 2)
	permuted := chronological(4, 8, 2, 1)

	var values []float64
	for _, data := range [][]market.Candle{candles, permuted} {
		result, err := c.Evaluate(context.Background(), analysis.Input{Candles: map[analysis.Unit][]market.Candle{analysis.UnitDays: data}})
		if err != nil {
			t.Fatal(err)
		}
		values = append(values, result.Metrics["range_percent"])
	}
	if values[0] != 2 || values[1] != values[0] {
		t.Fatalf("range percent by range order = %v", values)
	}
}

func TestPercentileCalculationIsIdenticalAcrossCandleUnits(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	data := []market.Candle{candle(start, 8), candle(start.Add(time.Hour), 1), candle(start.Add(2*time.Hour), 4), candle(start.Add(3*time.Hour), 2)}

	for _, unit := range []analysis.Unit{analysis.UnitDays, analysis.UnitHours} {
		c, err := volatility.New().Build(map[string]any{"unit": string(unit), "period": float64(4), "percentile": float64(75), "minimum_range_percent": float64(0)})
		if err != nil {
			t.Fatal(err)
		}
		result, err := c.Evaluate(context.Background(), analysis.Input{Candles: map[analysis.Unit][]market.Candle{unit: data}})
		if err != nil || result.Metrics["range_percent"] != 2 {
			t.Fatalf("unit=%s result=%+v err=%v", unit, result, err)
		}
	}
}

func TestHigherPercentileReturnsLowerRange(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	data := []market.Candle{candle(start, 1), candle(start.AddDate(0, 0, 1), 2), candle(start.AddDate(0, 0, 2), 4), candle(start.AddDate(0, 0, 3), 8)}
	input := analysis.Input{Candles: map[analysis.Unit][]market.Candle{analysis.UnitDays: data}}

	values := make(map[float64]float64)
	for _, percentile := range []float64{25, 75} {
		c, err := volatility.New().Build(map[string]any{"unit": "days", "period": float64(4), "percentile": percentile, "minimum_range_percent": float64(0)})
		if err != nil {
			t.Fatal(err)
		}
		result, err := c.Evaluate(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		values[percentile] = result.Metrics["range_percent"]
	}
	if values[75] != 2 || values[25] != 8 || values[75] >= values[25] {
		t.Fatalf("range percent by percentile = %v", values)
	}
}
func TestPercentileUsesNewestConfiguredHistoryAndReportsIt(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	for _, test := range []struct {
		unit analysis.Unit
		step time.Duration
	}{
		{unit: analysis.UnitDays, step: 24 * time.Hour},
		{unit: analysis.UnitHours, step: time.Hour},
	} {
		t.Run(string(test.unit), func(t *testing.T) {
			c, err := volatility.New().Build(map[string]any{"unit": string(test.unit), "period": float64(2), "percentile": float64(50), "minimum_range_percent": float64(0)})
			if err != nil {
				t.Fatal(err)
			}
			data := []market.Candle{
				candle(start, 99),
				candle(start.Add(test.step), 1),
				candle(start.Add(2*test.step), 3),
			}
			result, err := c.Evaluate(context.Background(), analysis.Input{Candles: map[analysis.Unit][]market.Candle{test.unit: data}})
			if err != nil || result.Metrics["range_percent"] != 3 || result.CandleCount != 2 || !result.From.Equal(start.Add(test.step)) || !result.To.Equal(start.Add(2*test.step)) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestPercentileRejectsInsufficientHistory(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		unit      analysis.Unit
		period    int
		available int
	}{
		{name: "zero daily history", unit: analysis.UnitDays, period: 2, available: 0},
		{name: "partial daily history", unit: analysis.UnitDays, period: 30, available: 13},
		{name: "partial hourly history", unit: analysis.UnitHours, period: 30, available: 13},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, err := volatility.New().Build(map[string]any{"unit": string(test.unit), "period": float64(test.period), "percentile": float64(50), "minimum_range_percent": float64(0)})
			if err != nil {
				t.Fatal(err)
			}
			data := make([]market.Candle, test.available)
			for i := range data {
				data[i] = candle(start.Add(time.Duration(i)*time.Hour), 1)
			}
			_, err = c.Evaluate(context.Background(), analysis.Input{Candles: map[analysis.Unit][]market.Candle{test.unit: data}})
			var insufficient *analysis.InsufficientHistoryError
			if !errors.As(err, &insufficient) || insufficient.Criterion != "volatility" || insufficient.Required != test.period || insufficient.Available != test.available {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestPercentileRejectsInvalidCandleWithSufficientHistory(t *testing.T) {
	c, err := volatility.New().Build(map[string]any{"unit": "days", "period": float64(2), "percentile": float64(50), "minimum_range_percent": float64(0)})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	data := []market.Candle{candle(start, 1), candle(start.AddDate(0, 0, 1), 2)}
	data[0].Open = 0
	_, err = c.Evaluate(context.Background(), analysis.Input{Candles: map[analysis.Unit][]market.Candle{analysis.UnitDays: data}})
	if !errors.Is(err, analysis.ErrInvalidCandleData) {
		t.Fatalf("err=%v", err)
	}
}
func TestPercentileRejectsInvalidParameters(t *testing.T) {
	_, err := volatility.New().Build(map[string]any{"unit": "days", "period": float64(1), "percentile": math.NaN(), "minimum_range_percent": float64(0)})
	if !errors.Is(err, analysis.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
}
func candle(t time.Time, r float64) market.Candle {
	return market.Candle{OpenTime: t, Open: 100, High: 100 + r, Low: 100}
}
