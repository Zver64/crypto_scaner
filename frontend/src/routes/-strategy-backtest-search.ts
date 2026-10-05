import {
	type MarketScanSortSearch,
	parseMarketScanSortSearch,
	parseSymbolFilterSearch,
	type SymbolFilterSearch,
} from "@/routes/-market-scan-search";

// Shared by the backtest coin list and coin pages: the list sort and filter and
// the chosen strategy are carried between them.
export interface StrategyBacktestSearch
	extends MarketScanSortSearch,
		SymbolFilterSearch {
	strategy?: number;
}

export function parseStrategyBacktestSearch(
	search: Record<string, unknown>,
): StrategyBacktestSearch {
	const strategy = search.strategy;
	return {
		...parseMarketScanSortSearch(search),
		...parseSymbolFilterSearch(search),
		...(typeof strategy === "number" &&
		Number.isSafeInteger(strategy) &&
		strategy > 0
			? { strategy }
			: {}),
	};
}
