import type { BacktestStrategyQueryError } from "@/api/generated/api";
import type { StrategyBacktest } from "@/api/generated/models";
import type {
	BacktestCoinResult,
	BacktestRunParameters,
} from "@/features/strategy-backtest/types";

export function normalizeBacktestSymbols(values: readonly string[]): string[] {
	return [
		...new Set(
			values.map((value) => value.trim().toUpperCase()).filter(Boolean),
		),
	];
}

export function backtestFailure(error: unknown): {
	message: string;
	stop: boolean;
} {
	const failure =
		error instanceof Error ? (error as BacktestStrategyQueryError) : undefined;
	const status = failure?.status;
	const code = failure?.info?.error?.code;
	const message =
		failure?.info?.error?.message ?? "The backtest could not be run.";
	return {
		message,
		stop:
			status === undefined ||
			status === 401 ||
			status === 403 ||
			code === "backtest_busy" ||
			code === "strategy_not_found" ||
			(status >= 500 && code !== "backtest_too_heavy"),
	};
}

export async function runBacktests(
	parameters: BacktestRunParameters,
	signal: AbortSignal,
	request: (
		symbol: string,
		chart: boolean,
		signal: AbortSignal,
	) => Promise<StrategyBacktest>,
	update: (items: BacktestCoinResult[]) => void,
): Promise<void> {
	let items: BacktestCoinResult[] = parameters.symbols.map((symbol) => ({
		symbol,
		status: "queued",
	}));
	const publish = () => update(items.map((item) => ({ ...item })));
	publish();
	for (let index = 0; index < items.length; index++) {
		const item = items[index];
		if (signal.aborted) break;
		item.status = "running";
		publish();
		try {
			const result = await request(item.symbol, items.length === 1, signal);
			if (signal.aborted) {
				item.status = "cancelled";
				break;
			}
			item.result = result;
			item.status = "completed";
		} catch (error) {
			if (signal.aborted) {
				item.status = "cancelled";
				break;
			}
			const failure = backtestFailure(error);
			item.error = failure.message;
			item.status = "failed";
			if (failure.stop) break;
		}
		publish();
	}
	items = items.map((item) =>
		item.status === "queued" || item.status === "running"
			? { ...item, status: signal.aborted ? "cancelled" : "not_run" }
			: item,
	);
	publish();
}
