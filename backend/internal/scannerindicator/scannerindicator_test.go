package scannerindicator_test

import (
	"log/slog"
	"slices"
	"testing"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
)

// unusedStore satisfies the Store interface for calls that never persist.
type unusedStore struct{ scannerindicator.Store }

func TestPreviewPlacesIndicatorsByTheirSourceUnits(t *testing.T) {
	registry, err := indicator.NewRegistry(talib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	service, err := scannerindicator.New(unusedStore{}, registry, []string{"yellow.5"}, slog.New(slog.DiscardHandler), func() {}, func() map[int64][]string { return nil })
	if err != nil {
		t.Fatal(err)
	}
	source := func(field string) float64 { return float64(slices.Index(indicator.CandleFields, field)) }
	tests := []struct {
		name          string
		indicatorType indicator.Type
		field         string
		want          chart.Placement
	}{
		{name: "overlay of prices", indicatorType: talib.EMAType, field: "close", want: chart.PlacementOverlay},
		{name: "overlay of volumes", indicatorType: talib.EMAType, field: "volume", want: chart.PlacementVolume},
		{name: "overlay of quote volumes", indicatorType: talib.EMAType, field: "quote_asset_volume", want: chart.PlacementPane},
		{name: "overlay of trade counts", indicatorType: talib.EMAType, field: "trade_count", want: chart.PlacementPane},
		{name: "oscillator of volumes", indicatorType: talib.RSIType, field: "volume", want: chart.PlacementPane},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, err := service.Preview(scannerindicator.Indicator{
				Interval:  market.IntervalDay,
				Selection: indicator.Selection{Type: test.indicatorType, Parameters: indicator.Parameters{"source": source(test.field)}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if entry.Placement != test.want {
				t.Fatalf("placement = %q, want %q", entry.Placement, test.want)
			}
		})
	}
}
