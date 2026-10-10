import { describe, expect, it } from "vitest";
import type { BacktestChart } from "@/api/generated/models";
import { createBacktestChartData } from "@/features/strategy-backtest/chart-data";

function chart(interval: "1h" | "1d", length: number): BacktestChart {
	const step = interval === "1h" ? 3_600_000 : 86_400_000;
	const candles = Array.from({ length }, (_, i) => ({
		open_time: new Date(Date.UTC(2025, 0, 1) + i * step).toISOString(),
		close_time: new Date(
			Date.UTC(2025, 0, 1) + (i + 1) * step - 1,
		).toISOString(),
		quote_asset_volume: 100,
		trade_count: 10,
		open: i,
		high: i + 1,
		low: i - 1,
		close: i,
		volume: 10,
	}));
	return {
		catalog: [
			{
				id: "rsi",
				type: "rsi",
				parameters: {},
				placement: "pane",
				pane: "rsi",
				scale: { levels: [] },
				lines: [{ output: "rsi", title: "RSI", color: "blue.5" }],
			},
		],
		page: {
			symbol: "BTCUSDT",
			interval,
			candles,
			has_more: false,
			indicators: [
				{
					type: "rsi",
					parameters: {},
					series: [
						{
							name: "rsi",
							points: candles.map((c) => ({
								time: c.open_time,
								value: c.close,
							})),
						},
					],
				},
			],
		},
	};
}

describe("backtest snapshot source", () => {
	it("scrolls to the beginning beyond 2000 candles, keeping lines aligned and intervals independent", () => {
		const hourly = chart("1h", 2501);
		const daily = chart("1d", 40);
		const source = createBacktestChartData([hourly, daily]);
		let changes = 0;
		const unsubscribe = source.subscribe(() => {
			changes++;
		});
		source.start();
		source.show("1h");
		const day = source.getSnapshot("1d");
		const initial = source.getSnapshot("1h");
		expect(initial.candles.at(-1)).toEqual(hourly.page.candles.at(-1));
		expect(initial.candles.length).toBeLessThan(hourly.page.candles.length);
		while (source.getSnapshot("1h").hasMore) source.loadOlder("1h");
		const full = source.getSnapshot("1h");
		expect(full.candles).toEqual(hourly.page.candles);
		expect(full.indicators.rsi?.rsi).toEqual(
			hourly.page.indicators[0]?.series[0]?.points,
		);
		expect(source.getSnapshot("1d")).toBe(day);
		expect(initial.candles.length).toBeLessThan(full.candles.length);
		expect(changes).toBeGreaterThan(0);
		const completed = changes;
		source.loadOlder("1h");
		source.stop();
		source.start();
		expect(changes).toBe(completed);
		expect(source.getSnapshot("1h")).toBe(full);
		unsubscribe();
	});
	it("handles an empty run without loading or claiming older history", () => {
		const source = createBacktestChartData([]);
		expect(source.getSnapshot("1h")).toMatchObject({
			candles: [],
			indicators: {},
			isLoading: false,
			hasMore: false,
		});
		source.loadOlder("1h");
		expect(source.getSnapshot("1h").candles).toEqual([]);
	});
});
