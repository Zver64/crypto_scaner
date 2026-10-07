import { backtestHold } from "@/features/strategy-backtest/constants";

// The single backtest page keeps its selected strategy, coin, coin filter and
// hold in the URL. Favorites only is the default; no hold uses the backend
// default of the strategy's interval.
export interface StrategyBacktestSearch {
	all_coins?: true;
	hold?: number;
	strategy?: number;
	symbol?: string;
}

function isIntegerIn(
	value: unknown,
	min: number,
	max: number,
): value is number {
	return (
		typeof value === "number" &&
		Number.isSafeInteger(value) &&
		value >= min &&
		value <= max
	);
}

export function parseStrategyBacktestSearch(
	search: Record<string, unknown>,
): StrategyBacktestSearch {
	const { hold, strategy } = search;
	const symbol =
		typeof search.symbol === "string" ? search.symbol.trim().toUpperCase() : "";
	return {
		...(search.all_coins === true ? { all_coins: true } : {}),
		...(isIntegerIn(hold, backtestHold.min, backtestHold.max) ? { hold } : {}),
		...(isIntegerIn(strategy, 1, Number.MAX_SAFE_INTEGER) ? { strategy } : {}),
		...(symbol ? { symbol } : {}),
	};
}
