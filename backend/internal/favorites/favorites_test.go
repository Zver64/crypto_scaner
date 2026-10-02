package favorites

import (
	"context"
	"testing"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	indicatortalib "crypto-scanner/internal/indicator/talib"
	"crypto-scanner/internal/markettable"
)

type storeStub struct{}

func (storeStub) ListFavorites(context.Context, int64) ([]Favorite, error) {
	return []Favorite{
		{InstrumentID: 1, Symbol: "BTCUSDT", Active: true},
		{InstrumentID: 2, Symbol: "ETHUSDT", Active: true},
		{InstrumentID: 3, Symbol: "OLDUSDT"},
	}, nil
}
func (storeStub) AddFavorite(context.Context, int64, string) (Favorite, error) {
	return Favorite{}, nil
}
func (storeStub) RemoveFavorite(context.Context, int64, int64, string, bool) (int, error) {
	return 0, nil
}

type analyzerStub struct{ symbols []string }

func (stub *analyzerStub) SearchSymbols(_ context.Context, _ analysis.SearchRequest, symbols []string) (analysis.SearchResult, error) {
	stub.symbols = append([]string(nil), symbols...)
	return analysis.SearchResult{}, nil
}

// noConfiguredColumns is a table without admin-configured indicator columns.
type noConfiguredColumns struct{}

func (noConfiguredColumns) TableColumns() []markettable.Column { return nil }

type closedStub struct{}

func (closedStub) Latest(context.Context, []int64) map[int64][]closedindicator.Value { return nil }

func TestAnalyzeOrchestratesFavoriteSelectionOutsideHTTP(t *testing.T) {
	registry, err := indicator.NewRegistry(indicatortalib.New()...)
	if err != nil {
		t.Fatal(err)
	}
	table, err := markettable.NewCatalog(registry, noConfiguredColumns{}, markettable.Sort{Column: "alert_count", Direction: markettable.Descending},
		markettable.Column{ID: "symbol", Title: "Symbol", Kind: markettable.KindText, Source: markettable.Symbol{}},
		markettable.Column{ID: "alert_count", Title: "Alerts", Kind: markettable.KindCount, Sortable: true, Source: markettable.AlertCount{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &analyzerStub{}
	service := New(storeStub{}, 1, nil, analyzer, closedStub{}, table)
	result, err := service.Analyze(context.Background(), 42, analysis.SearchRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(analyzer.symbols) != 2 || analyzer.symbols[0] != "BTCUSDT" || analyzer.symbols[1] != "ETHUSDT" {
		t.Fatalf("analyzed symbols = %v", analyzer.symbols)
	}
	if len(result.Table.Rows) != 3 || result.Table.Rows[2].Symbol != "OLDUSDT" {
		t.Fatalf("table rows = %+v, want every favorite", result.Table.Rows)
	}
}
