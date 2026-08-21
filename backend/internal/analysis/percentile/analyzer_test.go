package percentile_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/analysis/percentile"
	"crypto-scanner/internal/market"
)

func TestAnalyzerInterpolatesTypeSevenPercentile(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	candles := []market.Candle{
		candleWithRange(start, 1),
		candleWithRange(start.AddDate(0, 0, 1), 2),
		candleWithRange(start.AddDate(0, 0, 2), 4),
		candleWithRange(start.AddDate(0, 0, 3), 8),
	}

	result, err := percentile.New().Analyze(context.Background(), analysis.AnalysisInput{
		Candles: candles,
		Unit:    analysis.UnitDays, Period: 4,
		Percentile: 75,
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.RangePercent != 5 {
		t.Fatalf("Analyze() range = %v, want 5", result.RangePercent)
	}
}

func TestAnalyzerReportsInsufficientHistory(t *testing.T) {
	t.Parallel()

	_, err := percentile.New().Analyze(context.Background(), analysis.AnalysisInput{
		Candles: []market.Candle{candleWithRange(time.Now(), 2)},
		Unit:    analysis.UnitDays, Period: 2,
		Percentile: 50,
	})
	var insufficient *analysis.InsufficientHistoryError
	if !errors.As(err, &insufficient) {
		t.Fatalf("Analyze() error = %v, want InsufficientHistoryError", err)
	}
	if insufficient.Required != 2 || insufficient.Available != 1 {
		t.Fatalf("insufficient history = %+v, want required 2 and available 1", insufficient)
	}
}

func TestAnalyzerRejectsNonPositiveOpen(t *testing.T) {
	t.Parallel()

	for _, open := range []float64{0, -1} {
		open := open
		t.Run(fmt.Sprintf("open_%g", open), func(t *testing.T) {
			t.Parallel()
			_, err := percentile.New().Analyze(context.Background(), analysis.AnalysisInput{
				Candles: []market.Candle{{
					OpenTime: time.Now(),
					Open:     open,
					High:     10,
					Low:      5,
				}},
				Unit: analysis.UnitDays, Period: 1,
				Percentile: 50,
			})
			if !errors.Is(err, analysis.ErrInvalidCandleData) {
				t.Fatalf("Analyze() error = %v, want ErrInvalidCandleData", err)
			}
		})
	}
}

func TestAnalyzerRejectsOutOfRangeInput(t *testing.T) {
	t.Parallel()

	candle := candleWithRange(time.Now(), 2)
	inputs := map[string]analysis.AnalysisInput{
		"zero period":          {Candles: []market.Candle{candle}, Unit: analysis.UnitDays, Period: 0, Percentile: 50},
		"period above maximum": {Candles: []market.Candle{candle}, Unit: analysis.UnitDays, Period: 3651, Percentile: 50},
		"negative percentile":  {Candles: []market.Candle{candle}, Unit: analysis.UnitDays, Period: 1, Percentile: -1},
		"percentile above 100": {Candles: []market.Candle{candle}, Unit: analysis.UnitDays, Period: 1, Percentile: 101},
	}

	for name, input := range inputs {
		input := input
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := percentile.New().Analyze(context.Background(), input)
			if !errors.Is(err, analysis.ErrInvalidArgument) {
				t.Fatalf("Analyze() error = %v, want ErrInvalidArgument", err)
			}
		})
	}
}

func TestAnalyzerSupportsPercentileEndpointsAndSingleValue(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	for name, test := range map[string]struct {
		candles    []market.Candle
		percentile float64
		want       float64
	}{
		"minimum":      {[]market.Candle{candleWithRange(start, 8), candleWithRange(start.AddDate(0, 0, 1), 1)}, 0, 1},
		"maximum":      {[]market.Candle{candleWithRange(start, 1), candleWithRange(start.AddDate(0, 0, 1), 8)}, 100, 8},
		"single value": {[]market.Candle{candleWithRange(start, 4)}, 37, 4},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result, err := percentile.New().Analyze(context.Background(), analysis.AnalysisInput{
				Candles: test.candles,
				Unit:    analysis.UnitDays, Period: len(test.candles),
				Percentile: test.percentile,
			})
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if result.RangePercent != test.want {
				t.Fatalf("Analyze() range = %v, want %v", result.RangePercent, test.want)
			}
		})
	}
}

func TestAnalyzerRetainsFullPrecision(t *testing.T) {
	t.Parallel()

	result, err := percentile.New().Analyze(context.Background(), analysis.AnalysisInput{
		Candles: []market.Candle{{
			OpenTime: time.Now(),
			Open:     100_000,
			High:     101_234.56,
			Low:      100_000,
		}},
		Unit: analysis.UnitDays, Period: 1,
		Percentile: 50,
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if math.Abs(result.RangePercent-1.23456) > 1e-12 {
		t.Fatalf("Analyze() range = %.10f, want 1.23456", result.RangePercent)
	}
	if result.RangePercent == math.Round(result.RangePercent*10_000)/10_000 {
		t.Fatalf("Analyze() range = %.10f, unexpectedly rounded to four decimals", result.RangePercent)
	}
}

func TestAnalyzerSelectsLatestRequestedCandlesAndReportsCoverage(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	latest := start.AddDate(0, 0, 2)
	result, err := percentile.New().Analyze(context.Background(), analysis.AnalysisInput{
		Candles: []market.Candle{
			candleWithRange(start, 100),
			candleWithRange(latest, 4),
			candleWithRange(start.AddDate(0, 0, 1), 2),
		},
		Unit: analysis.UnitDays, Period: 2,
		Percentile: 50,
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.RangePercent != 3 {
		t.Errorf("Analyze() range = %v, want 3", result.RangePercent)
	}
	if result.CandleCount != 2 {
		t.Errorf("Analyze() candle count = %d, want 2", result.CandleCount)
	}
	if !result.From.Equal(start.AddDate(0, 0, 1)) {
		t.Errorf("Analyze() from = %v, want %v", result.From, start.AddDate(0, 0, 1))
	}
	if !result.To.Equal(latest) {
		t.Errorf("Analyze() to = %v, want %v", result.To, latest)
	}
}

func candleWithRange(openTime time.Time, rangePercent float64) market.Candle {
	return market.Candle{
		OpenTime: openTime,
		Open:     100,
		High:     100 + rangePercent,
		Low:      100,
	}
}
