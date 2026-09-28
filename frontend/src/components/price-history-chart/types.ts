import type {
	CandlestickData,
	HistogramData,
	LineData,
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
	indicator: readonly IndicatorPoint[];
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
	loadOlder(interval: ChartInterval): void;
}
export interface ChartIntervalOption {
	label: string;
	value: ChartInterval;
	showTime?: boolean;
}
export interface ChartIndicatorOptions {
	bounds: { min: number; max: number };
	lines: readonly { price: number; title: string }[];
	formatValue(value: number): string;
	minMove: number;
}
export interface ChartReadoutOptions {
	label: string;
	format(candle: Pick<PriceCandle, "open" | "high" | "low" | "close">): string;
}
export interface PriceHistoryChartProps {
	enabled: boolean;
	indicator?: ChartIndicatorOptions;
	extraReadout?: ChartReadoutOptions;
	paperPadding: string;
	source: PriceHistorySource;
	symbol: string;
}

export type ChartCandle = CandlestickData<UTCTimestamp>;
export type ChartCandleSlot = ChartCandle | WhitespaceData<UTCTimestamp>;
export type ChartVolumeSlot =
	| HistogramData<UTCTimestamp>
	| WhitespaceData<UTCTimestamp>;
export type ChartIndicatorSlot =
	| LineData<UTCTimestamp>
	| WhitespaceData<UTCTimestamp>;
