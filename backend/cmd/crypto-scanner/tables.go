package main

import (
	"crypto-scanner/internal/analysis"
	marketcapcriterion "crypto-scanner/internal/analysis/criteria/marketcap"
	"crypto-scanner/internal/analysis/criteria/volatility"
	"crypto-scanner/internal/markettable"
)

// marketTableColumns are the columns of every market table, in display order.
// Clients render them from the analysis responses, so adding a column needs no
// frontend change unless it introduces a new kind. The administrator's
// indicator columns fill the ConfiguredIndicators slot and are tracked in the
// background. Criterion columns read the evaluation of that criterion,
// whatever key the client gave it.
func marketTableColumns() []markettable.Column {
	return []markettable.Column{
		{ID: "symbol", Title: "Symbol", Kind: markettable.KindText, Source: markettable.Symbol{}},
		{ID: "market_cap_usd", Title: "MCap", Kind: markettable.KindUSDCompact, Sortable: true,
			Source: markettable.CriterionMetric{Criterion: marketcapcriterion.Name, Metric: marketcapcriterion.USDMetric}},
		{ID: "daily_range_percent", Title: "D Range", Kind: markettable.KindRangePercent, Sortable: true,
			Source: markettable.CriterionMetric{Criterion: volatility.Name, Unit: analysis.UnitDays, Metric: volatility.RangePercentMetric}},
		{ID: "hourly_range_percent", Title: "H Range", Kind: markettable.KindRangePercent, Sortable: true,
			Source: markettable.CriterionMetric{Criterion: volatility.Name, Unit: analysis.UnitHours, Metric: volatility.RangePercentMetric}},
		{ID: "seven_day_change_percent", Title: "7d %", Kind: markettable.KindPercentChange, Sortable: true, Source: markettable.PriceChangePercent{}},
		{ID: "price_history", Title: "7d chart", Kind: markettable.KindSparkline, Source: markettable.PriceHistory{}},
		{ID: "binance", Title: "Binance", Kind: markettable.KindLink, Source: markettable.ExchangeLink{}},
		{Source: markettable.ConfiguredIndicators{}},
		{ID: "favorite", Title: "Favorite", Kind: markettable.KindFavorite, Source: markettable.Favorite{}},
	}
}

var defaultTableSort = markettable.Sort{Column: "market_cap_usd", Direction: markettable.Descending}

// favoritesTableColumns extend the market table with the user's alert count.
func favoritesTableColumns() []markettable.Column {
	return append(marketTableColumns(), markettable.Column{ID: "alert_count", Title: "Alerts", Kind: markettable.KindCount, Source: markettable.AlertCount{}})
}
