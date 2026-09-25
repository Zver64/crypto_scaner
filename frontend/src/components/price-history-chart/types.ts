export type ChartInterval = string;
export interface PriceCandle {
	open_time: string;
	open: number;
	high: number;
	low: number;
	close: number;
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
	intervals: readonly [ChartIntervalOption, ...ChartIntervalOption[]];
	formatTime(time: string | number, interval: ChartInterval): string;
	nextOpen(time: string, interval: ChartInterval): string;
	extraReadout?: ChartReadoutOptions;
	paperPadding: string;
	source: PriceHistorySource;
	symbol: string;
}
