package strategy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"
)

// A strategy may read indicators nobody configured: it compiles, trades,
// and backtests all the same, without keeping them configured.
func TestStrategyReadsIndicatorsThatAreNotConfigured(t *testing.T) {
	registry := testRegistry(t)
	indicators := &previewIndicators{registry: registry}
	store := newBacktestStore(market.IntervalHour, nil)
	store.strategies = []Strategy{{ID: 1, Name: "Trend", Expression: "h_close > h_ema_3", StopLossExpression: "h_ema_5"}}
	service, err := NewService(store, indicators, registry, 7, slog.New(slog.DiscardHandler), func([]int64) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Load(context.Background()); err != nil {
		t.Fatal(err)
	}

	entry := service.List()[0]
	if entry.Compiled == nil || entry.StopLoss == nil {
		t.Fatalf("strategy did not compile: %s", entry.Problem)
	}
	if titles := entryTitles(service.Unconfigured(entry)); fmt.Sprint(titles) != "[h-ema-3 h-ema-5]" {
		t.Fatalf("unconfigured = %v", titles)
	}
	if ids := entry.indicatorIDs(); len(ids) != 0 {
		t.Fatalf("indicator ids = %v, want none linked", ids)
	}
	if _, err := service.Backtest(context.Background(), 1, "BTCUSDT", backtestStart, backtestStart); err != nil {
		t.Fatalf("backtest: %v", err)
	}

	indicators.entries = []scannerindicator.Entry{testEMA(t, registry, 4, market.IntervalHour, 3)}
	if titles := entryTitles(service.Unconfigured(entry)); fmt.Sprint(titles) != "[h-ema-5]" {
		t.Fatalf("unconfigured after adding h-ema-3 = %v", titles)
	}
	if err := service.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	entry = service.List()[0]
	if ids := entry.indicatorIDs(); fmt.Sprint(ids) != "[4]" {
		t.Fatalf("indicator ids after reloading = %v, want [4]", ids)
	}

	// Removing an indicator the strategy compiled with makes it missing
	// again.
	indicators.entries = nil
	if titles := entryTitles(service.Unconfigured(entry)); fmt.Sprint(titles) != "[h-ema-3 h-ema-5]" {
		t.Fatalf("unconfigured after removing h-ema-3 = %v", titles)
	}
}

func TestValidateResolvesIndicatorsThatAreNotConfigured(t *testing.T) {
	registry := testRegistry(t)
	service, err := NewService(newBacktestStore(market.IntervalHour, nil), &previewIndicators{registry: registry}, registry, 7, slog.New(slog.DiscardHandler), func([]int64) {})
	if err != nil {
		t.Fatal(err)
	}

	validation, err := service.Validate(context.Background(), "h_ema_3 > h_ema_5 && h_unknown > 0", EntryRule)
	if err != nil {
		t.Fatal(err)
	}
	if titles := entryTitles(validation.Missing); fmt.Sprint(titles) != "[h-ema-3 h-ema-5]" {
		t.Fatalf("missing = %v", titles)
	}
	if len(validation.Problems) != 1 {
		t.Fatalf("problems = %v, want only the unknown name", validation.Problems)
	}

	validation, err = service.Validate(context.Background(), "h_ema_3 > h_ema_5", EntryRule)
	if err != nil {
		t.Fatal(err)
	}
	if len(validation.Problems) != 0 || len(validation.Missing) != 2 {
		t.Fatalf("validation = %+v, want valid with two missing indicators", validation)
	}
}

// previewIndicators configures entries and previews EMAs titled the way the
// indicator service titles them.
type previewIndicators struct {
	entries  []scannerindicator.Entry
	registry *indicator.Registry
}

func (indicators *previewIndicators) List() []scannerindicator.Entry { return indicators.entries }

func (indicators *previewIndicators) Preview(item scannerindicator.Indicator) (scannerindicator.Entry, error) {
	if item.Selection.Type != "ema" {
		return scannerindicator.Entry{}, errors.New("not previewed")
	}
	selection, err := indicators.registry.Normalize(item.Selection)
	if err != nil {
		return scannerindicator.Entry{}, err
	}
	item.Selection = selection
	title := fmt.Sprintf("%s-ema-%v", scannerindicator.IntervalPrefix(item.Interval), selection.Parameters["period"])
	return scannerindicator.Entry{Indicator: item, Title: title, Outputs: []string{"ema"}}, nil
}

func entryTitles(entries []scannerindicator.Entry) []string {
	titles := make([]string, len(entries))
	for i, entry := range entries {
		titles[i] = entry.Title
	}
	return titles
}
