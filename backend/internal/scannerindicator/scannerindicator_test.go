package scannerindicator_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"

	"crypto-scanner/internal/chart"
	"crypto-scanner/internal/chart/draw"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
)

// unusedStore satisfies the Store interface for calls that never persist.
type unusedStore struct{ scannerindicator.Store }

func newService(store scannerindicator.Store, registry *indicator.Registry) (*scannerindicator.Service, error) {
	drawers, err := chart.NewDrawers(registry, chart.Palette{Colors: []string{"yellow.5"}, Bullish: "green.6", Bearish: "red.6"}, draw.Default{}, nil)
	if err != nil {
		return nil, err
	}
	return scannerindicator.New(store, registry, drawers, slog.New(slog.DiscardHandler), func() {})
}

// Only previews take an indicator of another indicator, which is titled
// after its source.
func TestIndicatorsOfIndicatorsAreOnlyPreviewed(t *testing.T) {
	registry, err := indicator.NewRegistry(talib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	service, err := newService(unusedStore{}, registry)
	if err != nil {
		t.Fatal(err)
	}
	rsi := indicator.Source{Selection: indicator.Selection{Type: talib.RSIType}, Output: "rsi"}
	selection := indicator.Selection{Type: "sma", Parameters: indicator.Parameters{"source": rsi.Value()}}
	entry, err := service.Preview(selection)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Title != "sma-rsi" || entry.Placement != chart.PlacementPane {
		t.Errorf("sma of rsi is titled %q and placed %q, want sma-rsi in a pane", entry.Title, entry.Placement)
	}
	if _, err := service.Create(context.Background(), scannerindicator.Indicator{Selection: selection}); !errors.Is(err, scannerindicator.ErrInvalidArgument) {
		t.Errorf("Create() of sma of rsi error = %v, want ErrInvalidArgument", err)
	}
}

// memoryStore keeps created indicators in memory.
type memoryStore struct {
	scannerindicator.Store
	next int64
}

func (store *memoryStore) CreateScannerIndicator(context.Context, scannerindicator.Indicator) (int64, error) {
	store.next++
	return store.next, nil
}

func (store *memoryStore) UpdateScannerIndicator(context.Context, scannerindicator.Indicator) error {
	return nil
}

// One indicator is shown per period: a table column and a chart line of
// each interval that shows it, and charts count it against their own limit.
func TestIndicatorPeriods(t *testing.T) {
	registry, err := indicator.NewRegistry(talib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	service, err := newService(&memoryStore{}, registry)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rsi := func(period int) indicator.Selection {
		return indicator.Selection{Type: talib.RSIType, Parameters: indicator.Parameters{"period": period}}
	}
	entry, err := service.Create(ctx, scannerindicator.Indicator{Selection: rsi(21), Periods: []scannerindicator.Period{
		{Interval: market.IntervalDay, ShowInTable: true, ShowInChart: true}, {Interval: market.IntervalHour, ShowInTable: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, column := range service.TableColumns() {
		titles = append(titles, column.ID+" "+column.Title)
	}
	if want := []string{"indicator_1_1h h-rsi-21", "indicator_1_1d d-rsi-21"}; !slices.Equal(titles, want) {
		t.Fatalf("columns = %v, want %v", titles, want)
	}
	if len(service.ChartCatalog(market.IntervalDay)) != 1 || len(service.ChartCatalog(market.IntervalHour)) != 0 {
		t.Fatal("charts draw the indicator outside its chart periods")
	}
	if _, err := service.Create(ctx, scannerindicator.Indicator{Selection: rsi(21)}); !errors.Is(err, scannerindicator.ErrConflict) {
		t.Fatalf("duplicate selection error = %v, want ErrConflict", err)
	}

	weekly := []scannerindicator.Period{{Interval: market.IntervalWeek, ShowInChart: true}}
	for period := range chart.MaxIndicators - 1 {
		if _, err := service.Create(ctx, scannerindicator.Indicator{Selection: rsi(2 + period), Periods: weekly}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.Update(ctx, entry.ID, weekly, scannerindicator.Scale{}); err != nil {
		t.Fatalf("the last chart slot: %v", err)
	}
	if _, err := service.Create(ctx, scannerindicator.Indicator{Selection: rsi(30), Periods: weekly}); !errors.Is(err, scannerindicator.ErrLimit) {
		t.Fatalf("chart limit error = %v, want ErrLimit", err)
	}
}
