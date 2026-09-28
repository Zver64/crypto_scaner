import { alpha, type MantineColorScheme } from "@mantine/core";
import {
	type CandlestickSeriesPartialOptions,
	type DeepPartial,
	type HistogramSeriesPartialOptions,
	type LineSeriesPartialOptions,
	LineStyle,
	type PriceLineOptions,
	type PriceScaleOptions,
	type TimeChartOptions,
} from "lightweight-charts";
import { resolvedTheme } from "@/app/theme";
import type {
	ChartInterval,
	ChartIntervalOption,
} from "@/components/price-history-chart/types";

export const chartIntervalOptions: readonly [
	ChartIntervalOption,
	...ChartIntervalOption[],
] = [
	{ label: "Hourly", value: "1h", showTime: true },
	{ label: "Daily", value: "1d" },
	{ label: "Weekly", value: "1w" },
	{ label: "Monthly", value: "1M" },
];

export const defaultChartInterval: ChartInterval = "1d";

export const chartHeight = 300;
// Candle pane and indicator pane heights, in that order.
export const paneStretchFactors = [3, 1] as const;
// Request older candles when fewer than this many bars remain on the left.
export const loadOlderThreshold = 10;
export const minVisibleBars = 24;

const { colors, white } = resolvedTheme;
const upColor = colors.green[6];
const downColor = colors.red[6];

// The resolved scheme returned by useColorScheme.
type ColorScheme = Exclude<MantineColorScheme, "auto">;

// Colors that depend on the light or dark color scheme.
const schemeColors = {
	dark: {
		background: colors.dark[7],
		grid: colors.dark[5],
		text: colors.dark[0],
	},
	light: { background: white, grid: colors.gray[3], text: colors.gray[7] },
};
type SchemeColors = (typeof schemeColors)[ColorScheme];

// Translucent so the volume bars stay behind the candles visually.
export const volumeColors = {
	down: alpha(downColor, 0.4),
	up: alpha(upColor, 0.4),
} as const;

export const barSpacing = 7.5;

function chartOptionsFor({ background, grid, text }: SchemeColors) {
	return {
		grid: { horzLines: { color: grid }, vertLines: { color: grid } },
		layout: { background: { color: background }, textColor: text },
		rightPriceScale: { borderColor: grid },
		timeScale: {
			barSpacing,
			borderColor: grid,
			minBarSpacing: 2,
			secondsVisible: false,
		},
	} satisfies DeepPartial<TimeChartOptions>;
}

export const chartOptions = {
	dark: chartOptionsFor(schemeColors.dark),
	light: chartOptionsFor(schemeColors.light),
} satisfies Record<ColorScheme, DeepPartial<TimeChartOptions>>;

export const candleSeriesOptions = {
	borderVisible: false,
	downColor,
	upColor,
	wickDownColor: downColor,
	wickUpColor: upColor,
} satisfies CandlestickSeriesPartialOptions;

export const candlePriceScaleOptions = {
	// Library defaults leave 20% empty above and 10% below the candles.
	scaleMargins: { bottom: 0.05, top: 0.05 },
} satisfies DeepPartial<PriceScaleOptions>;

export const volumeSeriesOptions = {
	lastValueVisible: false,
	priceFormat: { type: "volume" },
	priceLineVisible: false,
	// An empty id puts the series on its own hidden overlay price scale.
	priceScaleId: "",
} satisfies HistogramSeriesPartialOptions;

export const volumePriceScaleOptions = {
	// Volume bars use the bottom 20% of the candle pane, like TradingView.
	scaleMargins: { bottom: 0, top: 0.8 },
} satisfies DeepPartial<PriceScaleOptions>;

export const indicatorSeriesOptions = {
	color: colors.blue[5],
	lastValueVisible: true,
	lineWidth: 2,
	priceLineVisible: false,
} satisfies LineSeriesPartialOptions;

export const indicatorPriceScaleOptions = {
	autoScale: true,
	scaleMargins: { bottom: 0, top: 0 },
} satisfies DeepPartial<PriceScaleOptions>;

function priceLineOptionsFor(color: string) {
	return {
		axisLabelVisible: true,
		color,
		lineStyle: LineStyle.Dashed,
		lineWidth: 1,
	} satisfies Partial<PriceLineOptions>;
}

// Labeled RSI levels, such as 30 and 70.
export const indicatorPriceLineOptions = {
	dark: priceLineOptionsFor(schemeColors.dark.grid),
	light: priceLineOptionsFor(schemeColors.light.grid),
} satisfies Record<ColorScheme, Partial<PriceLineOptions>>;

// Axis labels for the visible minimum and maximum prices, without a line.
export const minMaxPriceLineOptions = {
	dark: { ...priceLineOptionsFor(schemeColors.dark.text), lineVisible: false },
	light: {
		...priceLineOptionsFor(schemeColors.light.text),
		lineVisible: false,
	},
} satisfies Record<ColorScheme, Partial<PriceLineOptions>>;
