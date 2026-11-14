package markettable_test

import (
	"context"
	"math"
	"slices"
	"testing"

	"crypto-scanner/internal/analysis"
	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/favorites"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/markettable"
)

type analyzerStub struct{ symbols []string }

func (*analyzerStub) Search(context.Context, analysis.SearchRequest) (analysis.SearchResult, error) {
	return analysis.SearchResult{}, nil
}

func (stub *analyzerStub) SearchSymbols(_ context.Context, _ analysis.SearchRequest, symbols []string) (analysis.SearchResult, error) {
	stub.symbols = append([]string(nil), symbols...)
	items := make([]analysis.SearchItem, len(symbols))
	for i, symbol := range symbols {
		items[i] = analysis.SearchItem{Symbol: symbol}
	}
	return analysis.SearchResult{Items: items}, nil
}

// PriceHistories gives each instrument a seven-day series whose closes are
// its id.
func (*analyzerStub) PriceHistories(_ context.Context, ids []int64, _ market.PriceWindows) (map[int64]analysis.PriceHistories, error) {
	histories := map[int64]analysis.PriceHistories{}
	for _, id := range ids {
		closes := make([]*float64, market.SevenDayPriceSlots)
		for slot := range closes {
			closes[slot] = new(float64(id))
		}
		histories[id] = analysis.PriceHistories{SevenDay: closes}
	}
	return histories, nil
}

// noConfiguredColumns is a table without admin-configured indicator columns.
type noConfiguredColumns struct{}

func (noConfiguredColumns) TableColumns() []markettable.Column { return nil }

type closedStub struct{}

type perpetualsStub []string

func (stub perpetualsStub) ListCoinMPerpetuals(context.Context) ([]string, error) { return stub, nil }

func (closedStub) Latest(context.Context, []int64) map[int64][]closedindicator.Value { return nil }

func TestFavoritesAnalyzesActiveFavoritesAndKeepsEveryRow(t *testing.T) {
	table, err := markettable.NewCatalog(noConfiguredColumns{}, markettable.Sort{Column: "alert_count", Direction: markettable.Descending},
		markettable.Column{ID: "symbol", Title: "Symbol", Kind: markettable.KindText, Source: markettable.Symbol{}},
		markettable.Column{ID: "alert_count", Title: "Alerts", Kind: markettable.KindCount, Sortable: true, Source: markettable.AlertCount{}},
		markettable.Column{ID: "price_history", Title: "7d chart", Kind: markettable.KindSparkline, Source: markettable.PriceHistory{Period: markettable.SevenDays}},
	)
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &analyzerStub{}
	service, err := markettable.NewService(analyzer, closedStub{}, perpetualsStub{}, table, table, table)
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
	// A favorite the analysis does not return keeps its price history.
	if series := result.Table.Rows[2].Cells["price_history"].Series; len(series) == 0 || series[0] == nil || *series[0] != 3 {
		t.Fatalf("price history = %v, want the closes of the favorite", series)
	}
}

func TestCoinMAnalyzesSpotUSDTPairsOfPerpetualCoins(t *testing.T) {
	table, err := markettable.NewCatalog(noConfiguredColumns{}, markettable.Sort{Column: "alert_count", Direction: markettable.Descending},
		markettable.Column{ID: "alert_count", Title: "Alerts", Kind: markettable.KindCount, Sortable: true, Source: markettable.AlertCount{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &analyzerStub{}
	service, err := markettable.NewService(analyzer, closedStub{}, perpetualsStub{"BTC", "ETH"}, table, table, table)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CoinM(context.Background(), analysis.SearchRequest{}); err != nil {
		t.Fatal(err)
	}
	if len(analyzer.symbols) != 2 || analyzer.symbols[0] != "BTCUSDT" || analyzer.symbols[1] != "ETHUSDT" {
		t.Fatalf("analyzed symbols = %v", analyzer.symbols)
	}
}

func TestFavoritesMarkTheMarketsOfEveryCoin(t *testing.T) {
	table, err := markettable.NewCatalog(noConfiguredColumns{}, markettable.Sort{Column: "alert_count", Direction: markettable.Descending},
		markettable.Column{ID: "alert_count", Title: "Alerts", Kind: markettable.KindCount, Sortable: true, Source: markettable.AlertCount{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	service, err := markettable.NewService(&analyzerStub{}, closedStub{}, perpetualsStub{"BTC", "OLD"}, table, table, table)
	if err != nil {
		t.Fatal(err)
	}
	items := []favorites.Favorite{
		{InstrumentID: 1, Symbol: "BTCUSDT", Active: true},
		{InstrumentID: 2, Symbol: "ETHUSDT", Active: true},
		{InstrumentID: 3, Symbol: "OLDUSDT"},
	}
	result, err := service.Favorites(context.Background(), items, analysis.SearchRequest{})
	if err != nil {
		t.Fatal(err)
	}
	coinM := []markettable.Market{markettable.MarketSpot, markettable.MarketCoinMPerpetual}
	want := [][]markettable.Market{coinM, {markettable.MarketSpot}, coinM}
	for i, row := range result.Table.Rows {
		if !slices.Equal(row.Markets, want[i]) {
			t.Errorf("%s markets = %v, want %v", row.Symbol, row.Markets, want[i])
		}
	}
}

func TestMetricRatioIsUnavailableWithoutAPositiveDenominator(t *testing.T) {
	daily := markettable.CriterionMetric{Criterion: "volatility", Unit: analysis.UnitDays, Metric: "range_percent"}
	hourly := markettable.CriterionMetric{Criterion: "volatility", Unit: analysis.UnitHours, Metric: "range_percent"}
	catalog, err := markettable.NewCatalog(noConfiguredColumns{}, markettable.Sort{Column: "ratio", Direction: markettable.Descending},
		markettable.Column{ID: "ratio", Title: "Ratio", Kind: markettable.KindNumber, Sortable: true, Source: markettable.MetricRatio{Numerator: daily, Denominator: hourly}})
	if err != nil {
		t.Fatal(err)
	}
	evaluations := func(dailyRange, hourlyRange float64) []analysis.Evaluation {
		return []analysis.Evaluation{
			{Name: "volatility", Unit: analysis.UnitDays, Metrics: map[string]float64{"range_percent": dailyRange}},
			{Name: "volatility", Unit: analysis.UnitHours, Metrics: map[string]float64{"range_percent": hourlyRange}},
		}
	}
	table := catalog.Build([]markettable.Row{
		{Symbol: "RATIO", Evaluations: evaluations(6, 1.5)},
		{Symbol: "FLAT", Evaluations: evaluations(6, 0)},
		{Symbol: "MISSING", Evaluations: evaluations(6, 1.5)[:1]},
	}, market.PriceWindows{})
	if value := table.Rows[0].Cells["ratio"].Value; value == nil || *value != 4 {
		t.Fatalf("ratio = %v, want 4", value)
	}
	for _, row := range table.Rows[1:] {
		if value := row.Cells["ratio"].Value; value != nil {
			t.Fatalf("%s ratio = %v, want unavailable", row.Symbol, *value)
		}
	}
}

type configuredColumns []markettable.Column

func (columns configuredColumns) TableColumns() []markettable.Column { return columns }

func TestClosedIndicatorCellIsUnavailableWhenTheValueIsEmpty(t *testing.T) {
	target := closedindicator.Target{Interval: market.IntervalHour, Selection: indicator.Selection{Type: "pivot"}}
	column := markettable.Column{ID: "high", Title: "High", Kind: markettable.KindNumber, Source: markettable.ClosedIndicator{Target: target, Output: "high"}}
	catalog, err := markettable.NewCatalog(configuredColumns{column}, markettable.Sort{Column: "alert_count", Direction: markettable.Descending},
		markettable.Column{ID: "alert_count", Title: "Alerts", Kind: markettable.KindCount, Sortable: true, Source: markettable.AlertCount{}},
		markettable.Column{Source: markettable.ConfiguredIndicators{}})
	if err != nil {
		t.Fatal(err)
	}
	row := func(symbol string, value float64) markettable.Row {
		return markettable.Row{Symbol: symbol, ClosedIndicators: []closedindicator.Value{{Target: target, Outputs: []closedindicator.Output{{Name: "high", Value: value}}}}}
	}
	table := catalog.Build([]markettable.Row{row("A", 5), row("B", math.NaN())}, market.PriceWindows{})
	if value := table.Rows[0].Cells["high"].Value; value == nil || *value != 5 {
		t.Fatalf("pivot = %v, want 5", value)
	}
	if value := table.Rows[1].Cells["high"].Value; value != nil {
		t.Fatalf("empty = %v, want unavailable", *value)
	}
}
