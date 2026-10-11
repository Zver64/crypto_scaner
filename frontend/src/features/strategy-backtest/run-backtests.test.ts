import { describe, expect, it } from "vitest";
import type { StrategyBacktest } from "@/api/generated/models";
import {
	normalizeBacktestSymbols,
	runBacktests,
} from "@/features/strategy-backtest/run-backtests";
import type { BacktestCoinResult } from "@/features/strategy-backtest/types";

const result: StrategyBacktest = {
	symbol: "BTCUSDT",
	interval: "1h",
	direction: "long",
	fee: 0.001,
	duration_ms: 10,
	from: null,
	to: null,
	trades: [],
	charts: [],
	equity: [],
	indicator_columns: [],
	skipped_alerts: 0,
	signal: null,
	baselines: { buy_and_hold: null, dca: null },
	summary: {
		net_profit: 0,
		max_drawdown: 0,
		stats: {
			trade_count: 0,
			win_rate: null,
			profit_factor: null,
			average_trade: null,
			average_win: null,
			average_loss: null,
			average_bars: null,
			take_profit_exits: 0,
			stop_loss_exits: 0,
			exit_rule_exits: 0,
		},
	},
};
const parameters = {
	strategy: 3,
	symbols: ["BTCUSDT", "ETHUSDT", "ARBUSDT"],
	from: undefined,
	to: undefined,
};

describe("runBacktests", () => {
	it("normalizes explicit symbols, deduplicating in selection order", () => {
		expect(
			normalizeBacktestSymbols([" btcusdt ", "ETHUSDT", "", "BTCUSDT"]),
		).toEqual(["BTCUSDT", "ETHUSDT"]);
	});
	it("awaits each chart-free replay and continues coin-specific failures", async () => {
		const calls: string[] = [];
		const snapshots: BacktestCoinResult[][] = [];
		let release: (result: StrategyBacktest) => void = () => {};
		const first = new Promise<StrategyBacktest>((resolve) => {
			release = resolve;
		});
		const pending = runBacktests(
			parameters,
			new AbortController().signal,
			async (symbol, chart) => {
				calls.push(symbol);
				expect(chart).toBe(false);
				if (symbol === "BTCUSDT") return first;
				if (symbol === "ETHUSDT")
					throw Object.assign(new Error(), {
						status: 404,
						info: {
							error: { code: "symbol_not_found", message: "Unknown coin" },
						},
					});
				return result;
			},
			(items) => snapshots.push(items),
		);
		expect(calls).toEqual(["BTCUSDT"]);
		expect(snapshots[0][0].status).toBe("queued");
		release(result);
		await pending;
		expect(calls).toEqual(parameters.symbols);
		expect(snapshots.at(-1)?.map((item) => item.status)).toEqual([
			"completed",
			"failed",
			"completed",
		]);
		expect(snapshots.at(-1)?.[1].error).toBe("Unknown coin");
		expect(snapshots[0][0].status).toBe("queued");
	});
	it.each([
		[404, "strategy_not_found"],
		[401, "unauthenticated"],
		[403, "administrator_required"],
		[429, "backtest_busy"],
		[500, "internal_error"],
		[503, "unavailable"],
	])("stops queued coins on global failure %s/%s", async (status, code) => {
		const calls: string[] = [];
		let final: BacktestCoinResult[] = [];
		await runBacktests(
			parameters,
			new AbortController().signal,
			async (symbol) => {
				calls.push(symbol);
				throw Object.assign(new Error(), {
					status,
					info: { error: { code, message: "Stopped" } },
				});
			},
			(items) => {
				final = items;
			},
		);
		expect(calls).toEqual(["BTCUSDT"]);
		expect(final.map((item) => item.status)).toEqual([
			"failed",
			"not_run",
			"not_run",
		]);
	});
	it("continues a coin timeout but stops transport failures", async () => {
		const calls: string[] = [];
		let final: BacktestCoinResult[] = [];
		await runBacktests(
			parameters,
			new AbortController().signal,
			async (symbol) => {
				calls.push(symbol);
				if (symbol === "BTCUSDT")
					throw Object.assign(new Error(), {
						status: 503,
						info: {
							error: { code: "backtest_too_heavy", message: "Too heavy" },
						},
					});
				throw new TypeError("Network failure");
			},
			(items) => {
				final = items;
			},
		);
		expect(calls).toEqual(["BTCUSDT", "ETHUSDT"]);
		expect(final.map((item) => item.status)).toEqual([
			"failed",
			"failed",
			"not_run",
		]);
	});
	it("cancels the active request, discards its late success, and stops the queue", async () => {
		const controller = new AbortController();
		const calls: string[] = [];
		let final: BacktestCoinResult[] = [];
		await runBacktests(
			parameters,
			controller.signal,
			async (symbol) => {
				calls.push(symbol);
				controller.abort();
				return result;
			},
			(items) => {
				final = items;
			},
		);
		expect(calls).toEqual(["BTCUSDT"]);
		expect(final.map((item) => item.status)).toEqual([
			"cancelled",
			"cancelled",
			"cancelled",
		]);
		expect(final[0].result).toBeUndefined();
	});
	it("requests a chart for a single coin", async () => {
		let requested = false;
		await runBacktests(
			{ ...parameters, symbols: ["BTCUSDT"] },
			new AbortController().signal,
			async (_symbol, chart) => {
				requested = chart;
				return result;
			},
			() => {},
		);
		expect(requested).toBe(true);
	});
});
