package closedindicator

import (
	"slices"
	"testing"
	"time"

	"crypto-scanner/internal/market"
)

func TestMergedAppliesChangesByOpenTime(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bar := func(hour int, close float64) market.Candle {
		return market.Candle{OpenTime: start.Add(time.Duration(hour) * time.Hour), Close: close}
	}
	for _, test := range []struct {
		name            string
		window, changes []market.Candle
		depth           int
		want            []market.Candle
	}{
		{"appends and trims", []market.Candle{bar(0, 1), bar(1, 1)}, []market.Candle{bar(2, 2)}, 2, []market.Candle{bar(1, 1), bar(2, 2)}},
		{"replaces a correction", []market.Candle{bar(0, 1), bar(1, 1)}, []market.Candle{bar(0, 5)}, 2, []market.Candle{bar(0, 5), bar(1, 1)}},
		{"fills a gap", []market.Candle{bar(0, 1), bar(2, 1)}, []market.Candle{bar(1, 2)}, 3, []market.Candle{bar(0, 1), bar(1, 2), bar(2, 1)}},
		{"keeps older candles of a short window", []market.Candle{bar(2, 1)}, []market.Candle{bar(0, 2), bar(1, 2)}, 5, []market.Candle{bar(0, 2), bar(1, 2), bar(2, 1)}},
		{"drops older candles of a full window", []market.Candle{bar(1, 1), bar(2, 1)}, []market.Candle{bar(0, 2)}, 2, []market.Candle{bar(1, 1), bar(2, 1)}},
		{"keeps the later of repeated changes", nil, []market.Candle{bar(0, 1), bar(0, 2)}, 2, []market.Candle{bar(0, 2)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			window := slices.Clone(test.window)
			if got := merged(window, test.changes, test.depth); !slices.Equal(got, test.want) || !slices.Equal(window, test.window) {
				t.Fatalf("merged() = %v, window %v; want %v, window unchanged", got, window, test.want)
			}
		})
	}
}
