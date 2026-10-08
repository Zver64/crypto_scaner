import type {
	CandlestickData,
	HistogramData,
	LineData,
	SeriesMarkerBar,
	UTCTimestamp,
	WhitespaceData,
} from "lightweight-charts";

export type ChartInterval = "1h" | "1d" | "1w" | "1M";
export interface PriceCandle {
	open_time: string;
	open: number;
	high: number;
	low: number;
	close: number;
	volume: number;
}
export interface IndicatorPoint {
	time: string;
	value: number;
}
export type ChartConnection = "connecting" | "connected" | "disconnected";
export type ChartFreshness = "waiting" | "fresh" | "stale" | "recovering";

export interface PriceHistorySnapshot {
	candles: readonly PriceCandle[];
	indicators: ChartIndicatorPoints;
	connection: ChartConnection;
	freshness: ChartFreshness;
	error?: string;
	hasMore: boolean;
	isLoading: boolean;
	isLoadingMore: boolean;
}
export interface PriceHistorySource {
	getSnapshot(interval: ChartInterval): PriceHistorySnapshot;
	subscribe(listener: () => void): () => void;
	start(): void;
	stop(): void;
	// Selects the interval on screen; the source streams only that interval.
	show(interval: ChartInterval): void;
	loadOlder(interval: ChartInterval): void;
}
export interface ChartIntervalOption {
	label: string;
	value: ChartInterval;
	showTime?: boolean;
}
// Points of each indicator line, keyed by indicator id and then output name.
export type ChartIndicatorPoints = Readonly<
	Record<string, Readonly<Record<string, readonly IndicatorPoint[]>>>
>;
export interface ChartIndicatorLine {
	output: string;
	title: string;
	color: string;
}
export interface ChartIndicatorScale {
	min?: number;
	max?: number;
	levels: readonly { value: number; title: string }[];
}
// Overlays share the candle pane and price scale; panes are drawn below the
// candles, and indicators with the same pane key share a pane and value scale.
export type ChartIndicatorOptions =
	| { id: string; placement: "overlay"; lines: readonly ChartIndicatorLine[] }
	| ChartPaneIndicatorOptions;
export interface ChartPaneIndicatorOptions {
	id: string;
	placement: "pane";
	pane: string;
	lines: readonly ChartIndicatorLine[];
	scale: ChartIndicatorScale;
}
// One pane below the candles with the indicators drawn in it.
export interface ChartIndicatorPane {
	indicators: readonly ChartPaneIndicatorOptions[];
	scale: ChartIndicatorScale;
}
// Entry and exit times of trades, arrows under and over their candles, and
// other marked times, circles over their candles.
export interface ChartTradeMarkers {
	entries: readonly string[];
	exits: readonly string[];
	marks?: readonly string[];
}
export interface ChartReadoutOptions {
	label: string;
	format(candle: Pick<PriceCandle, "open" | "high" | "low" | "close">): string;
}
export interface PriceHistoryChartProps {
	enabled: boolean;
	// Indicators drawn on the chart of each interval.
	indicators: Readonly<Record<ChartInterval, readonly ChartIndicatorOptions[]>>;
	extraReadout?: ChartReadoutOptions;
	// Stretches the chart over the free height of its flex column parent,
	// keeping its fixed height as the minimum.
	fillHeight?: boolean;
	// Restricts the interval switch to these intervals; the first is shown
	// first.
	intervals?: readonly ChartInterval[];
	// Trades marked with arrows on the candles containing their times.
	markers?: ChartTradeMarkers;
	paperPadding: string;
	source: PriceHistorySource;
	symbol: string;
}

export type ChartCandle = CandlestickData<UTCTimestamp>;
export type ChartCandleSlot = ChartCandle | WhitespaceData<UTCTimestamp>;
export type ChartMarker = SeriesMarkerBar<UTCTimestamp>;
export type ChartVolumeSlot =
	| HistogramData<UTCTimestamp>
	| WhitespaceData<UTCTimestamp>;
export type ChartIndicatorSlot =
	| LineData<UTCTimestamp>
	| WhitespaceData<UTCTimestamp>;

// One entry of the indicator legend above the chart.
export interface ChartLegendItem {
	key: string;
	placement: ChartIndicatorOptions["placement"];
	title: string;
	color: string;
	value: string | null;
}
