// The single backtest page keeps its selected strategy, coin and coin filter
// in the URL. Favorites only is the default.
export interface StrategyBacktestSearch {
	all_coins?: true;
	strategy?: number;
	symbol?: string;
}

export function parseStrategyBacktestSearch(
	search: Record<string, unknown>,
): StrategyBacktestSearch {
	const { strategy } = search;
	const symbol =
		typeof search.symbol === "string" ? search.symbol.trim().toUpperCase() : "";
	return {
		...(search.all_coins === true ? { all_coins: true } : {}),
		...(typeof strategy === "number" &&
		Number.isSafeInteger(strategy) &&
		strategy >= 1
			? { strategy }
			: {}),
		...(symbol ? { symbol } : {}),
	};
}
