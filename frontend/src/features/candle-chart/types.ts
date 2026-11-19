import type {
	AutoscaleInfoProvider,
	CandlestickData,
	HistogramData,
	LineData,
	SeriesMarkerBar,
	UTCTimestamp,
	WhitespaceData,
} from "lightweight-charts";

import type {
	CandleInterval,
	ChartIndicatorLevel,
	ChartIndicatorScale,
	ChartPlacement,
} from "@/api/generated/models";
import type { SeriesFillPoint } from "@/components/lightweight-chart";

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
	// The candles and indicator shapes ready to draw, created with the snapshot
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
	getSnapshot(interval: CandleInterval): PriceHistorySnapshot;
	subscribe(listener: () => void): () => void;
	// Selects the interval on screen; a live source streams only that interval.
	show(interval: CandleInterval): void;
	loadOlder(interval: CandleInterval): void;
}
// The price of another instrument, drawn over the candles as a line on its own
// left price scale.
export interface PriceComparison {
	label: string;
	// The line aligned with the candle slots of the interval, empty while the
	// comparison is disabled or has no data. It keeps its reference until its
	// data changes.
	getLine(interval: CandleInterval): ChartLineData;
	subscribe(listener: () => void): () => void;
	// The source streams the comparison only while it is enabled.
	setEnabled(enabled: boolean): void;
}
// Number resolution of the values on a price or value scale.
export interface ValueResolution {
	base: number;
	fractionDigits: number;
	minMove: number;
}
// A line aligned with the candle slots, such as the compared price.
export interface ChartLineData {
	data: readonly ChartIndicatorSlot[];
	// The resolution of the line's scale.
	resolution: ValueResolution;
}
export interface ChartSeriesData {
	// Candle slots, with whitespace for missing candles.
	candles: readonly ChartCandleSlot[];
	volumes: readonly ChartVolumeSlot[];
	priceResolution: ValueResolution;
	// The panes below the candles, top to bottom.
	panes: readonly ChartPaneData[];
	// The shapes the indicators are drawn with, in the order they arrive.
	shapes: readonly ChartShapeData[];
}
// One pane below the candles.
export interface ChartPaneData {
	key: string;
	// Fits the pane to its visible values and the scale levels. It is kept
	// from the previous snapshot while the scale does not change, so live
	// updates do not reconfigure the series of the pane.
	autoscaleInfoProvider: AutoscaleInfoProvider;
	scale: ChartIndicatorScale;
}
// Where a shape is drawn: over the candles on their price scale, over the
// volume bars on their scale, or in a pane below the candles, which has an
// autoscale of its own. Pane 0 holds the candles.
export interface ChartShapePlace {
	placement: ChartPlacement;
	pane: number;
	autoscaleInfoProvider?: AutoscaleInfoProvider;
}
// Colors are Mantine theme color tokens, resolved where they are drawn.
export type ChartShapeData =
	| ChartLineShapeData
	| ChartAreaShapeData
	| ChartMarkersShapeData
	| ChartSegmentsShapeData;
export interface ChartLineShapeData extends ChartShapePlace {
	kind: "line";
	id: string;
	title: string;
	color: string;
	// Aligned with the candle slots; a point with a color of its own carries
	// it.
	data: readonly ChartIndicatorSlot[];
	// The times of the points whose join to the next point is not drawn.
	gaps: ReadonlySet<UTCTimestamp>;
	// The resolution of the line's scale; an overlay shares the candles'.
	resolution: ValueResolution;
	// The levels of its pane, drawn by the first line of each pane.
	levels: readonly ChartIndicatorLevel[];
}
export interface ChartAreaShapeData extends ChartShapePlace {
	kind: "area";
	id: string;
	// The values of the area, as the data of the series the fill lies on.
	values: readonly ChartIndicatorSlot[];
	// Aligned with the candle slots.
	points: readonly SeriesFillPoint[];
}
export interface ChartMarkersShapeData extends ChartShapePlace {
	kind: "markers";
	id: string;
	title: string;
	color: string;
	position: ChartMarker["position"];
	// The value of each marker by the candle it is drawn on.
	values: ReadonlyMap<UTCTimestamp, number>;
	// The values of the line the markers on a line sit on, in time order, as
	// the data of the series that carries them.
	onLine: readonly LineData<UTCTimestamp>[];
	// The candles of the markers drawn on the candles, in time order.
	onCandles: readonly UTCTimestamp[];
}
export interface ChartSegmentsShapeData extends ChartShapePlace {
	kind: "segments";
	id: string;
	color: string;
	// The points of the segments in time order as the data of one line
	// series: segments sharing a point meet there, and the end of a segment
	// followed by a gap colors the join to the next segment transparent.
	data: readonly LineData<UTCTimestamp>[];
}
// Price levels drawn across the candles, such as the levels of a grid, with an
// autoscale provider that keeps them in view. Created with createChartPriceLevels.
export interface ChartPriceLevels {
	autoscaleInfoProvider: AutoscaleInfoProvider;
	// Ascending and distinct; the first and last are the bounds.
	levels: readonly number[];
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
export interface PriceHistoryChartProps {
	// Adds a toggle that draws the comparison line.
	comparison?: PriceComparison;
	// Immutable snapshots have no connection status or current live candle.
	live?: boolean;
	// Stretches the chart over the free height of its flex column parent,
	// keeping its fixed height as the minimum.
	fillHeight?: boolean;
	// Restricts the interval switch to these intervals; the first is shown
	// first.
	intervals?: readonly CandleInterval[];
	paperPadding: string;
	priceLevels?: ChartPriceLevels;
	source: PriceHistorySource;
	symbol: string;
}

export type ChartCandle = CandlestickData<UTCTimestamp>;
export type ChartCandleSlot = ChartCandle | WhitespaceData<UTCTimestamp>;
export type ChartMarker = SeriesMarkerBar<UTCTimestamp>;
// A price of a candle, such as its high.
export interface ChartPricePoint {
	price: number;
	time: UTCTimestamp;
}
// The lowest low and the highest high of the visible candles.
export interface ChartMinMax {
	max: ChartPricePoint;
	min: ChartPricePoint;
}
export type ChartMinMaxMarkerOptions = Record<
	keyof ChartMinMax,
	Omit<ChartMarker, "time">
>;
export type ChartVolumeSlot =
	| HistogramData<UTCTimestamp>
	| WhitespaceData<UTCTimestamp>;
export type ChartIndicatorSlot =
	| LineData<UTCTimestamp>
	| WhitespaceData<UTCTimestamp>;

// One entry of the indicator legend over the chart.
export interface ChartLegendItem {
	key: string;
	placement: ChartPlacement;
	title: string;
	color: string;
	value: string | null;
}
