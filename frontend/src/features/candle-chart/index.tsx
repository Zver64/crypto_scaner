import {
	Box,
	Center,
	Loader,
	Paper,
	SegmentedControl,
	Stack,
	Text,
	useComputedColorScheme,
} from "@mantine/core";
import type { DeepPartial, Time, TimeChartOptions } from "lightweight-charts";
import {
	type CSSProperties,
	memo,
	useCallback,
	useMemo,
	useState,
} from "react";
import { ChartCanvas } from "@/components/lightweight-chart";
import { CandleSeries } from "@/features/candle-chart/candle-series";
import { ChartReadout } from "@/features/candle-chart/chart-readout";
import {
	barSpacing,
	chartOptions as baseChartOptions,
	candlePaneHeight,
	chartIntervalOptions,
	defaultChartInterval,
	indicatorPaneHeight,
	loadOlderThreshold,
	minVisibleBars,
} from "@/features/candle-chart/config";
import { IndicatorLegend } from "@/features/candle-chart/indicator-legend";
import { LiveStatus } from "@/features/candle-chart/live-status";
import { OverlayIndicatorSeries } from "@/features/candle-chart/overlay-indicator-series";
import { PaneIndicatorSeries } from "@/features/candle-chart/pane-indicator-series";
import type {
	ChartCandle,
	ChartInterval,
	PriceHistoryChartProps,
} from "@/features/candle-chart/types";
import { useChartViewport } from "@/features/candle-chart/use-chart-viewport";
import { usePriceFormat } from "@/features/candle-chart/use-price-format";
import { usePriceHistory } from "@/features/candle-chart/use-price-history";
import {
	createCandlestickData,
	createIndicatorData,
	createIndicatorLegend,
	createIndicatorPanes,
	createMarkerDataSelector,
	formatChartTime,
} from "@/features/candle-chart/utils";
import { VolumeSeries } from "@/features/candle-chart/volume-series";

export type {
	ChartIndicatorOptions,
	ChartIndicatorPoints,
	ChartIntervalOption,
	ChartReadoutOptions,
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/features/candle-chart/types";

const intervalControlData = chartIntervalOptions.map(({ label, value }) => ({
	label,
	value,
}));

export const PriceHistoryChart = memo(function PriceHistoryChart({
	enabled,
	indicators: intervalIndicators,
	extraReadout,
	fillHeight = false,
	intervals,
	markers,
	paperPadding,
	source,
	symbol,
}: PriceHistoryChartProps) {
	const [interval, setInterval] = useState<ChartInterval>(
		intervals?.[0] ?? defaultChartInterval,
	);
	const intervalOptions = intervals
		? intervalControlData.filter(({ value }) => intervals.includes(value))
		: intervalControlData;
	const [activeCandle, setActiveCandle] = useState<ChartCandle | null>(null);
	const indicators = intervalIndicators[interval];
	const colorScheme = useComputedColorScheme("dark");
	const {
		candles,
		connection,
		error,
		freshness,
		hasMore,
		indicators: indicatorPoints,
		isLoading,
		isLoadingMore,
	} = usePriceHistory(source, interval, enabled);

	const data = useMemo(
		() => createCandlestickData(candles, interval),
		[candles, interval],
	);
	const priceFormat = usePriceFormat(data);
	const selectMarkerData = useMemo(
		() => markers && createMarkerDataSelector(markers, interval),
		[markers, interval],
	);
	const markerData = useMemo(
		() => selectMarkerData?.(data),
		[data, selectMarkerData],
	);
	// Aligns every indicator line with the candle slots, keyed by "id:output".
	const indicatorData = useMemo(
		() =>
			new Map(
				indicators.flatMap(({ id, lines }) =>
					lines.map(({ output }) => [
						`${id}:${output}`,
						createIndicatorData(data, indicatorPoints[id]?.[output] ?? []),
					]),
				),
			),
		[data, indicators, indicatorPoints],
	);
	const overlays = indicators.filter(
		(indicator) => indicator.placement === "overlay",
	);
	const panes = useMemo(() => createIndicatorPanes(indicators), [indicators]);
	const chartOptions = useMemo<DeepPartial<TimeChartOptions>>(
		() => ({
			...baseChartOptions[colorScheme],
			localization: {
				timeFormatter: (time: Time) =>
					typeof time === "number"
						? formatChartTime(time, interval)
						: String(time),
			},
			timeScale: {
				...baseChartOptions[colorScheme].timeScale,
				timeVisible:
					chartIntervalOptions.find((item) => item.value === interval)
						?.showTime ?? false,
			},
		}),
		[colorScheme, interval],
	);

	const onLoadOlder = useCallback(
		() => source.loadOlder(interval),
		[source, interval],
	);
	const {
		chartRef,
		onBeforeDataChange,
		onVisibleLogicalRangeChange,
		visibleRange,
	} = useChartViewport({
		barWidth: barSpacing,
		data,
		hasMore,
		isLoadingMore,
		minVisibleBars,
		onLoadOlder,
		threshold: loadOlderThreshold,
	});

	const selectInterval = (value: ChartInterval) => {
		setActiveCandle(null);
		setInterval(value);
	};
	const readoutCandle = activeCandle ?? candles.at(-1);
	const legendSlotIndex = activeCandle
		? data.findIndex((slot) => slot.time === activeCandle.time)
		: data.length - 1;
	const legendItems = createIndicatorLegend(
		indicators,
		indicatorData,
		legendSlotIndex,
	);
	const hasCandles = candles.length > 0;
	const chartHeight = candlePaneHeight + panes.length * indicatorPaneHeight;
	const fillStyle: CSSProperties | undefined = fillHeight
		? { display: "flex", flex: 1, flexDirection: "column" }
		: undefined;

	return (
		<Paper component="section" p={paperPadding} style={fillStyle}>
			<Stack gap="md" style={fillStyle}>
				<SegmentedControl<ChartInterval>
					data={intervalOptions}
					fullWidth
					onChange={selectInterval}
					value={interval}
				/>
				<Box pos="relative" style={fillStyle}>
					<LiveStatus
						connection={connection}
						error={error}
						freshness={freshness}
					/>
					{readoutCandle ? (
						<ChartReadout candle={readoutCandle} extra={extraReadout} />
					) : null}
					{readoutCandle ? <IndicatorLegend items={legendItems} /> : null}
					{!hasCandles && !isLoading ? (
						<Text c="dimmed">No closed candles are available.</Text>
					) : null}
					<ChartCanvas
						key={`${symbol}:${interval}`}
						aria-label={`${symbol}: ${interval} candlestick history with the current live candle. ${candles.length} candles loaded.${hasMore ? " Scroll left to load older candles." : " Earliest stored candle reached."}`}
						onVisibleLogicalRangeChange={onVisibleLogicalRangeChange}
						options={chartOptions}
						paneStretchFactors={[
							candlePaneHeight,
							...panes.map(() => indicatorPaneHeight),
						]}
						ref={chartRef}
						role="img"
						style={
							fillHeight
								? { flex: 1, minHeight: chartHeight, width: "100%" }
								: { height: chartHeight, width: "100%" }
						}
					>
						{/* Added first so the volume bars are drawn behind the candles. As the
						first series to receive data, it also records the viewport first. */}
						<VolumeSeries
							candles={candles}
							data={data}
							onBeforeDataChange={onBeforeDataChange}
						/>
						<CandleSeries
							data={data}
							markers={markerData}
							onActiveCandleChange={setActiveCandle}
							priceFormat={priceFormat}
							visibleRange={visibleRange}
						/>
						{overlays.flatMap(({ id, lines }) =>
							lines.map((line) => (
								<OverlayIndicatorSeries
									data={indicatorData.get(`${id}:${line.output}`) ?? []}
									key={`${id}:${line.output}`}
									line={line}
									priceFormat={priceFormat}
								/>
							)),
						)}
						{panes.flatMap(({ indicators: paneIndicators, scale }, paneIndex) =>
							paneIndicators.flatMap(({ id, lines }, indicatorIndex) =>
								lines.map((line, lineIndex) => (
									<PaneIndicatorSeries
										data={indicatorData.get(`${id}:${line.output}`) ?? []}
										drawsLevels={indicatorIndex === 0 && lineIndex === 0}
										key={`${id}:${line.output}`}
										line={line}
										pane={paneIndex + 1}
										scale={scale}
									/>
								)),
							),
						)}
					</ChartCanvas>
					{isLoading && !hasCandles ? (
						<Center inset={0} pos="absolute">
							<Loader aria-label={`Loading ${interval} candle history`} />
						</Center>
					) : null}
					{/* A filling chart keeps the line's place, so the canvas does not
					resize while older candles load. */}
					{isLoadingMore || fillHeight ? (
						<Text
							c="dimmed"
							size="xs"
							style={isLoadingMore ? undefined : { visibility: "hidden" }}
							ta="center"
						>
							Loading older candles…
						</Text>
					) : null}
				</Box>
			</Stack>
		</Paper>
	);
});
