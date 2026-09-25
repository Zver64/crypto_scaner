package favorites

import (
	"context"
	"testing"

	"crypto-scanner/internal/analysis"
)

type storeStub struct{}

func (storeStub) ListFavorites(context.Context, int64) ([]Favorite, error) { return nil, nil }
func (storeStub) ListFavoriteSymbols(context.Context, int64) ([]string, error) {
	return []string{"BTCUSDT", "ETHUSDT"}, nil
}
func (storeStub) AddFavorite(context.Context, int64, string) (Favorite, error) {
	return Favorite{}, nil
}
func (storeStub) RemoveFavorite(context.Context, int64, string, bool) (int, error) { return 0, nil }

type analyzerStub struct{ symbols []string }

func (stub *analyzerStub) SearchSymbols(_ context.Context, _ analysis.SearchRequest, symbols []string) (analysis.SearchResult, error) {
	stub.symbols = append([]string(nil), symbols...)
	return analysis.SearchResult{}, nil
}

func TestAnalyzeOrchestratesFavoriteSelectionOutsideHTTP(t *testing.T) {
	analyzer := &analyzerStub{}
	service := New(storeStub{}, nil, analyzer, nil)
	if _, err := service.Analyze(context.Background(), 42, analysis.SearchRequest{}); err != nil {
		t.Fatal(err)
	}
	if len(analyzer.symbols) != 2 || analyzer.symbols[0] != "BTCUSDT" || analyzer.symbols[1] != "ETHUSDT" {
		t.Fatalf("analyzed symbols = %v", analyzer.symbols)
	}
}
