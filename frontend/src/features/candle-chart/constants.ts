import type { ChartLineData } from "@/features/candle-chart/types";
import { valueResolution } from "@/features/candle-chart/utils";

// Creating an Intl.DateTimeFormat is expensive and chart readouts format a
// time on every crosshair move, so the formatters are created once.
export const hourTimeFormat = new Intl.DateTimeFormat("en", {
	day: "numeric",
	hour: "2-digit",
	hourCycle: "h23",
	minute: "2-digit",
	month: "short",
	timeZone: "UTC",
});
export const monthTimeFormat = new Intl.DateTimeFormat("en", {
	month: "short",
	year: "numeric",
	timeZone: "UTC",
});
export const dayTimeFormat = new Intl.DateTimeFormat("en", {
	day: "numeric",
	month: "short",
	year: "numeric",
	timeZone: "UTC",
});

// A line missing from a snapshot, such as one still loading, draws nothing.
export const emptyChartLine: ChartLineData = {
	data: [],
	resolution: valueResolution([]),
};
