import { describe, expect, it } from "vitest";
import type { SignalDirection } from "@/api/generated/models";
import {
	backtestMarkers,
	backtestPeriod,
	createEquityData,
	formatFractionPercent,
	hitsColor,
	shortenedPeriod,
} from "@/features/strategy-backtest/utils";

describe("formatFractionPercent", () => {
	it("formats fractions as percentages and a missing value as a dash", () => {
		expect(formatFractionPercent(-0.0123)).toBe("-1.23%");
		expect(formatFractionPercent(null)).toBe("—");
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
	const signal = (direction: SignalDirection) => ({
		signal: {
			direction,
			occurrences: [{ changes: [], close: 1, time, values: {} }],
			windows: [],
		},
		trades: [],
	});

	it("marks signals by the move they expect", () => {
		expect(backtestMarkers(signal("long"))).toEqual({
			entries: [time],
			exits: [],
			marks: [],
		});
		expect(backtestMarkers(signal("short"))).toEqual({
			entries: [],
			exits: [time],
			marks: [],
		});
		expect(backtestMarkers(signal("sideways"))).toEqual({
			entries: [],
			exits: [],
			marks: [time],
		});
	});
});

describe("hitsColor", () => {
	it("runs from red at no hits through yellow to green at every hit", () => {
		expect(hitsColor(0)).toMatch(/^hsl\(0 /);
		expect(hitsColor(0.5)).toMatch(/^hsl\(60 /);
		expect(hitsColor(1)).toMatch(/^hsl\(120 /);
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
