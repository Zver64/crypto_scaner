// The single backtest page keeps its selected strategy, coin, coin filter
// and period in the URL. Favorites only is the default.
export interface StrategyBacktestSearch {
	all_coins?: true;
	// The first and last UTC days of the evaluated period, as YYYY-MM-DD.
	from?: string;
	strategy?: number;
	symbol?: string;
	to?: string;
}

// A calendar date such as 2026-01-31.
const datePattern = /^\d{4}-\d{2}-\d{2}$/;

// A real calendar day: parsing rolls impossible days such as 2026-02-30
// over to another, which then reads back differently.
function searchDate(value: unknown): string | undefined {
	if (typeof value !== "string" || !datePattern.test(value)) return undefined;
	const time = Date.parse(`${value}T00:00:00Z`);
	return !Number.isNaN(time) &&
		new Date(time).toISOString().slice(0, 10) === value
		? value
		: undefined;
}

export function parseStrategyBacktestSearch(
	search: Record<string, unknown>,
): StrategyBacktestSearch {
	const { strategy } = search;
	const symbol =
		typeof search.symbol === "string" ? search.symbol.trim().toUpperCase() : "";
	const from = searchDate(search.from);
	const to = searchDate(search.to);
	return {
		...(search.all_coins === true ? { all_coins: true } : {}),
		...(from ? { from } : {}),
		...(to ? { to } : {}),
		...(typeof strategy === "number" &&
		Number.isSafeInteger(strategy) &&
		strategy >= 1
			? { strategy }
			: {}),
		...(symbol ? { symbol } : {}),
	};
}
