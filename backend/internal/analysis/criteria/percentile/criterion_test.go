package percentile_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/analysis/criteria/percentile"
	"crypto-scanner/internal/market"
)

func TestPercentileEvaluatesTypeSevenRangeAndThreshold(t *testing.T) {
	c, err := percentile.New().Build(map[string]any{"unit": "days", "period": float64(4), "percentile": float64(75), "minimum_range_percent": float64(5)})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	data := []market.Candle{candle(start, 1), candle(start.AddDate(0, 0, 1), 2), candle(start.AddDate(0, 0, 2), 4), candle(start.AddDate(0, 0, 3), 8)}
	result, err := c.Evaluate(context.Background(), map[analysis.Unit][]market.Candle{analysis.UnitDays: data})
	if err != nil || result.Metrics["range_percent"] != 5 || !result.Matched {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
func TestPercentileUsesAvailableHistoryAndReportsIt(t *testing.T) {
	c, err := percentile.New().Build(map[string]any{"unit": "days", "period": float64(2), "percentile": float64(50), "minimum_range_percent": float64(0)})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	result, err := c.Evaluate(context.Background(), map[analysis.Unit][]market.Candle{analysis.UnitDays: {candle(start, 1.23456)}})
	if err != nil || math.Abs(result.Metrics["range_percent"]-1.23456) > 1e-12 || result.CandleCount != 1 || !result.From.Equal(start) || !result.To.Equal(start) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestPercentileRejectsZeroHistory(t *testing.T) {
	c, err := percentile.New().Build(map[string]any{"unit": "days", "period": float64(2), "percentile": float64(50), "minimum_range_percent": float64(0)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Evaluate(context.Background(), map[analysis.Unit][]market.Candle{analysis.UnitDays: {}})
	var insufficient *analysis.InsufficientHistoryError
	if !errors.As(err, &insufficient) || insufficient.Criterion != "percentile" || insufficient.Required != 2 || insufficient.Available != 0 {
		t.Fatalf("err=%v", err)
	}
}
func TestPercentileRejectsInvalidParameters(t *testing.T) {
	_, err := percentile.New().Build(map[string]any{"unit": "days", "period": float64(1), "percentile": math.NaN(), "minimum_range_percent": float64(0)})
	if !errors.Is(err, analysis.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
}
func candle(t time.Time, r float64) market.Candle {
	return market.Candle{OpenTime: t, Open: 100, High: 100 + r, Low: 100}
}
