import type { StrategyID } from "@/api/generated/models";
import { normalizeBacktestSymbols } from "@/features/strategy-backtest/run-backtests";
import { strategyIdSchema } from "@/features/strategy-settings/schemas";

// The backtest page keeps its selected strategy, coins, coin filter
// and period in the URL. Favorites only is the default.
export interface StrategyBacktestSearch {
	all_coins?: true;
	// The first and last UTC days of the evaluated period, as YYYY-MM-DD.
	from?: string;
	strategy?: StrategyID;
	symbol?: string;
	symbols?: string[];
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
	const strategy = strategyIdSchema.safeParse(search.strategy);
	const symbols = normalizeBacktestSymbols(
		Array.isArray(search.symbols)
			? search.symbols.filter(
					(value): value is string => typeof value === "string",
				)
			: typeof search.symbol === "string"
				? [search.symbol]
				: [],
	);
	const from = searchDate(search.from);
	const to = searchDate(search.to);
	return {
		...(search.all_coins === true ? { all_coins: true } : {}),
		...(from ? { from } : {}),
		...(to ? { to } : {}),
		...(strategy.success ? { strategy: strategy.data } : {}),
		...(symbols.length > 1
			? { symbols }
			: symbols.length === 1
				? { symbol: symbols[0] }
				: {}),
	};
}
