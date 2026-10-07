import type {
	DeepPartial,
	LineSeriesPartialOptions,
	TimeChartOptions,
} from "lightweight-charts";
import { resolvedTheme } from "@/app/theme";

// Height of the equity chart in pixels.
export const equityChartHeight = 160;

// The small equity chart fits every trade and leaves page scrolling alone.
export const equityChartOptions = {
	handleScale: false,
	handleScroll: false,
	timeScale: { lockVisibleTimeRangeOnResize: true },
} satisfies DeepPartial<TimeChartOptions>;

export const equitySeriesOptions = {
	color: resolvedTheme.colors.teal[5],
	lineWidth: 2,
	priceLineVisible: false,
} satisfies LineSeriesPartialOptions;
