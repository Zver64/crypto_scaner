import { describe, expect, it } from "vitest";

import type {
	BacktestSignalOutcome,
	BacktestTrade,
	Direction,
} from "@/api/generated/models";
import {
	backtestMarkers,
	backtestPeriod,
	createEquityData,
	formatFractionPercent,
	formatSignalSuccesses,
	shortenedPeriod,
	successColor,
} from "@/features/strategy-backtest/utils";

describe("formatFractionPercent", () => {
	it("formats fractions as percentages and a missing value as a dash", () => {
		expect(formatFractionPercent(-0.0123)).toBe("-1.23%");
		expect(formatFractionPercent(null)).toBe("—");
	});
});

describe("formatSignalSuccesses", () => {
	it("excludes neutral outcomes from the displayed denominator", () => {
		expect(
			formatSignalSuccesses({
				count: 7,
				successes: 2,
				neutrals: 4,
				success_rate: 2 / 3,
				median_move: null,
			}),
		).toBe("2/3 (66.67%)");
	});
	it("shows a dash when no decisive outcomes exist", () => {
		expect(
			formatSignalSuccesses({
				count: 4,
				successes: 0,
				neutrals: 4,
				success_rate: null,
				median_move: null,
			}),
		).toBe("—");
	});
});

describe("successColor", () => {
	it("runs from red at no successes through yellow to green at every success", () => {
		expect(successColor(0)).toMatch(/^hsl\(0 /);
		expect(successColor(0.5)).toMatch(/^hsl\(60 /);
		expect(successColor(1)).toMatch(/^hsl\(120 /);
	});
});

describe("createEquityData", () => {
	it("starts at zero on the first evaluated candle and shows compounded net profit", () => {
		expect(
			createEquityData("2026-08-01T00:00:00Z", [
				{ equity: 1.1, time: "2026-08-03T00:00:00Z" },
				{ equity: 0.99, time: "2026-08-05T00:00:00Z" },
			]).map(({ time, value }) => [time, Number(value.toFixed(6))]),
		).toEqual([
			[Date.parse("2026-08-01T00:00:00Z") / 1_000, 0],
			[Date.parse("2026-08-03T00:00:00Z") / 1_000, 10],
			[Date.parse("2026-08-05T00:00:00Z") / 1_000, -1],
		]);
	});
});

describe("backtestMarkers", () => {
	const time = "2026-08-01T00:00:00Z";
	const signal = (direction: Direction) => ({
		direction,
		signal: {
			candles: 1,
			evaluation: null,
			occurrences: [
				{
					close: 1,
					counted: null,
					move: null,
					stop: null,
					exit_time: null,
					outcome: null,
					target: null,
					time,
					values: {},
					indicator_values: {},
				},
			],
		},
		trades: [],
	});

	it("keeps unevaluated directional signals as entries only", () => {
		for (const direction of ["long", "short"] as const) {
			const result = backtestMarkers(signal(direction));
			expect(result.groups?.map(({ times }) => times)).toEqual([
				[time],
				[],
				[],
				[],
			]);
		}

		for (const direction of ["sideways", "simple"] as const) {
			expect(backtestMarkers(signal(direction))).toEqual({
				entries: [],
				exits: [],
				marks: [time],
			});
		}
	});

	it("groups every evaluated exit by outcome, including uncounted repeats", () => {
		for (const direction of ["long", "short"] as const) {
			const draft = signal(direction);
			const outcomes: BacktestSignalOutcome[] = [
				"success",
				"failure",
				"neutral",
			];
			const exits = outcomes.map((_, i) => `2026-08-0${i + 2}T00:00:00Z`);
			const occurrences = outcomes.map((outcome, i) => ({
				...draft.signal.occurrences[0],
				outcome,
				exit_time: exits[i],
				counted: false,
			}));
			const result = backtestMarkers({
				...draft,
				signal: { ...draft.signal, occurrences },
			});
			expect(result.groups?.map(({ times }) => times)).toEqual([
				[time, time, time],
				...exits.map((exit) => [exit]),
			]);
		}
	});

	it("marks a short trade opening over its candle and closing under", () => {
		const trade = {
			buys: 1,
			entry_price: 10,
			entry_time: time,
			exit_price: 8,
			exit_reason: "take_profit",
			exit_signal_time: null,
			exit_time: "2026-08-02T00:00:00Z",
			exit_values: {},
			fills: [],
			net_return: 0.2,
			open: false,
			stop_loss: 12,
			take_profit: 8,
		} satisfies BacktestTrade;
		expect(
			backtestMarkers({ direction: "short", signal: null, trades: [trade] }),
		).toEqual({ entries: [trade.exit_time], exits: [time] });
	});
});

describe("backtestPeriod", () => {
	it("covers whole UTC days and leaves absent sides open", () => {
		expect(backtestPeriod("2026-01-01", "2026-03-31")).toEqual({
			from: "2026-01-01T00:00:00Z",
			to: "2026-03-31T23:59:59Z",
		});
		expect(backtestPeriod(undefined, undefined)).toEqual({});
	});
});

describe("shortenedPeriod", () => {
	const now = Date.parse("2026-10-08T20:30:00Z");
	const hourly = {
		from: "2026-08-04T00:00:00Z",
		interval: "1h",
		to: "2026-10-08T19:00:00Z",
	} as const;

	it("tells which side the evaluated candles fall short of the chosen days", () => {
		expect(
			shortenedPeriod({ from: "2026-01-28", to: "2026-12-31" }, hourly, now),
		).toEqual({ end: false, start: true });
		expect(
			shortenedPeriod({ from: undefined, to: "2026-10-07" }, hourly, now),
		).toEqual({ end: false, start: false });
		expect(
			shortenedPeriod(
				{ from: "2026-08-04", to: "2026-10-08" },
				{ ...hourly, from: "2026-08-04T16:00:00Z", to: "2026-10-08T12:00:00Z" },
				now,
			),
		).toEqual({ end: true, start: true });
	});

	it("does not count the candle still open now or a missing first day as covered", () => {
		expect(
			shortenedPeriod({ from: "2026-08-04", to: "2026-10-08" }, hourly, now),
		).toEqual({ end: false, start: false });
		expect(
			shortenedPeriod(
				{ from: "2026-08-04", to: undefined },
				{
					from: "2026-08-05T00:00:00Z",
					interval: "1d",
					to: "2026-10-07T00:00:00Z",
				},
				now,
			),
		).toEqual({ end: false, start: true });
	});

	it("counts whole candles of coarse intervals as covering their days", () => {
		expect(
			shortenedPeriod(
				{ from: "2026-09-02", to: "2026-10-04" },
				{
					from: "2026-09-07T00:00:00Z",
					interval: "1w",
					to: "2026-09-28T00:00:00Z",
				},
				now,
			),
		).toEqual({ end: false, start: false });
		expect(
			shortenedPeriod(
				{ from: undefined, to: "2026-09-30" },
				{
					from: "2026-01-01T00:00:00Z",
					interval: "1M",
					to: "2026-09-01T00:00:00Z",
				},
				now,
			),
		).toEqual({ end: false, start: false });
	});
});
