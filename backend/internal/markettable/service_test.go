package markettable_test

import (
	"context"
	"testing"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/favorites"
	"crypto-scanner/internal/markettable"
)

type analyzerStub struct{ symbols []string }

func (*analyzerStub) Search(context.Context, analysis.SearchRequest) (analysis.SearchResult, error) {
	return analysis.SearchResult{}, nil
}

func (stub *analyzerStub) SearchSymbols(_ context.Context, _ analysis.SearchRequest, symbols []string) (analysis.SearchResult, error) {
	stub.symbols = append([]string(nil), symbols...)
	return analysis.SearchResult{}, nil
}

// noConfiguredColumns is a table without admin-configured indicator columns.
type noConfiguredColumns struct{}

func (noConfiguredColumns) TableColumns() []markettable.Column { return nil }

type closedStub struct{}

func (closedStub) Latest(context.Context, []int64) map[int64][]closedindicator.Value { return nil }

func TestFavoritesAnalyzesActiveFavoritesAndKeepsEveryRow(t *testing.T) {
	table, err := markettable.NewCatalog(noConfiguredColumns{}, markettable.Sort{Column: "alert_count", Direction: markettable.Descending},
		markettable.Column{ID: "symbol", Title: "Symbol", Kind: markettable.KindText, Source: markettable.Symbol{}},
		markettable.Column{ID: "alert_count", Title: "Alerts", Kind: markettable.KindCount, Sortable: true, Source: markettable.AlertCount{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &analyzerStub{}
	service, err := markettable.NewService(analyzer, closedStub{}, table, table)
	if err != nil {
		t.Fatal(err)
	}
	items := []favorites.Favorite{
		{InstrumentID: 1, Symbol: "BTCUSDT", Active: true},
		{InstrumentID: 2, Symbol: "ETHUSDT", Active: true},
		{InstrumentID: 3, Symbol: "OLDUSDT", AlertCount: 2},
	}
	result, err := service.Favorites(context.Background(), items, analysis.SearchRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(analyzer.symbols) != 2 || analyzer.symbols[0] != "BTCUSDT" || analyzer.symbols[1] != "ETHUSDT" {
		t.Fatalf("analyzed symbols = %v", analyzer.symbols)
	}
	if len(result.Table.Rows) != 3 || result.Table.Rows[2].Symbol != "OLDUSDT" {
		t.Fatalf("table rows = %+v, want every favorite", result.Table.Rows)
	}
	if count := result.Table.Rows[2].Cells["alert_count"].Value; count == nil || *count != 2 {
		t.Fatalf("alert count = %v, want 2", count)
	}
}
