import type {
	AutoscaleInfoProvider,
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
	// The candles and indicator lines ready to draw, created with the snapshot
	// so they keep their references until the snapshot data changes.
	series: ChartSeriesData;
	// Markers on the candles, such as the trades of a backtest.
	markers?: readonly ChartMarker[];
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
// Number resolution of the values on a price or value scale.
export interface ValueResolution {
	base: number;
	fractionDigits: number;
	minMove: number;
}
// One indicator line aligned with the candle slots.
export interface ChartLineData {
	data: readonly ChartIndicatorSlot[];
	// The resolution of a pane line's scale; an overlay shares the candles'.
	resolution: ValueResolution;
}
export interface ChartSeriesData {
	// Candle slots, with whitespace for missing candles.
	candles: readonly ChartCandleSlot[];
	volumes: readonly ChartVolumeSlot[];
	priceResolution: ValueResolution;
	// Every line of the catalog, keyed by "id:output".
	lines: Readonly<Record<string, ChartLineData>>;
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
// What the chart of one interval draws besides the candles, created once per
// catalog. Line colors are Mantine theme colors, resolved where they are drawn.
export interface ChartIndicatorLayout {
	indicators: readonly ChartIndicatorOptions[];
	panes: readonly ChartIndicatorPane[];
}
// One pane below the candles with the indicators drawn in it.
export interface ChartIndicatorPane {
	// Fits the pane to its visible values and the scale levels; created with
	// the layout, so it keeps its identity while the catalog does not change.
	autoscaleInfoProvider: AutoscaleInfoProvider;
	indicators: readonly ChartPaneIndicatorOptions[];
	scale: ChartIndicatorScale;
}
// Entry and exit times of trades, arrows under and over their candles, and
// other marked times, circles over their candles.
export interface ChartMarkerGroup {
	times: readonly string[];
	options: Omit<ChartMarker, "time">;
}
export interface ChartTradeMarkers {
	groups?: readonly ChartMarkerGroup[];
	entries: readonly string[];
	exits: readonly string[];
	marks?: readonly string[];
}
export interface ChartReadoutOptions {
	label: string;
	format(candle: Pick<PriceCandle, "open" | "high" | "low" | "close">): string;
}
export interface PriceHistoryChartProps {
	// Immutable snapshots have no connection status or current live candle.
	live?: boolean;
	enabled: boolean;
	// Indicators drawn on the chart of each interval.
	indicators: Readonly<Record<ChartInterval, ChartIndicatorLayout>>;
	extraReadout?: ChartReadoutOptions;
	// Stretches the chart over the free height of its flex column parent,
	// keeping its fixed height as the minimum.
	fillHeight?: boolean;
	// Restricts the interval switch to these intervals; the first is shown
	// first.
	intervals?: readonly ChartInterval[];
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
