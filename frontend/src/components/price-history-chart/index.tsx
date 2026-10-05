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
import { memo, useCallback, useMemo, useState } from "react";
import { ChartCanvas } from "@/components/lightweight-chart";
import { CandleSeries } from "@/components/price-history-chart/candle-series";
import { ChartReadout } from "@/components/price-history-chart/chart-readout";
import {
	barSpacing,
	chartOptions as baseChartOptions,
	candlePaneHeight,
	chartIntervalOptions,
	defaultChartInterval,
	indicatorPaneHeight,
	loadOlderThreshold,
	minVisibleBars,
} from "@/components/price-history-chart/config";
import { IndicatorLegend } from "@/components/price-history-chart/indicator-legend";
import { LiveStatus } from "@/components/price-history-chart/live-status";
import { OverlayIndicatorSeries } from "@/components/price-history-chart/overlay-indicator-series";
import { PaneIndicatorSeries } from "@/components/price-history-chart/pane-indicator-series";
import type {
	ChartCandle,
	ChartInterval,
	PriceHistoryChartProps,
} from "@/components/price-history-chart/types";
import { useChartViewport } from "@/components/price-history-chart/use-chart-viewport";
import { usePriceFormat } from "@/components/price-history-chart/use-price-format";
import { usePriceHistory } from "@/components/price-history-chart/use-price-history";
import {
	createCandlestickData,
	createIndicatorData,
	createIndicatorLegend,
	createIndicatorPanes,
	formatChartTime,
} from "@/components/price-history-chart/utils";
import { VolumeSeries } from "@/components/price-history-chart/volume-series";

export type {
	ChartIndicatorOptions,
	ChartIndicatorPoints,
	ChartIntervalOption,
	ChartReadoutOptions,
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/components/price-history-chart/types";

const intervalControlData = chartIntervalOptions.map(({ label, value }) => ({
	label,
	value,
}));

export const PriceHistoryChart = memo(function PriceHistoryChart({
	enabled,
	indicators: intervalIndicators,
	extraReadout,
	paperPadding,
	source,
	symbol,
}: PriceHistoryChartProps) {
	const [interval, setInterval] = useState<ChartInterval>(defaultChartInterval);
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

	const selectInterval = (value: string) => {
		setActiveCandle(null);
		setInterval(value as ChartInterval);
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

	return (
		<Paper component="section" p={paperPadding}>
			<Stack gap="md">
				<SegmentedControl
					data={intervalControlData}
					fullWidth
					onChange={selectInterval}
					value={interval}
				/>
				<Box pos="relative">
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
						style={{
							height: candlePaneHeight + panes.length * indicatorPaneHeight,
							width: "100%",
						}}
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
						{panes.flatMap(
							({ indicators: paneIndicators, scale }, paneIndex) => {
								const paneLines = paneIndicators.flatMap(({ id, lines }) =>
									lines.map((line) => {
										const key = `${id}:${line.output}`;
										return { data: indicatorData.get(key) ?? [], key, line };
									}),
								);
								// The chart skips price lines of a series without values, so the
								// levels go on the first line that has some.
								const levelsLine =
									paneLines.find(({ data }) =>
										data.some((slot) => "value" in slot),
									) ?? paneLines[0];
								return paneLines.map(({ data, key, line }) => (
									<PaneIndicatorSeries
										data={data}
										drawsLevels={key === levelsLine?.key}
										key={key}
										line={line}
										pane={paneIndex + 1}
										scale={scale}
									/>
								));
							},
						)}
					</ChartCanvas>
					{isLoading && !hasCandles ? (
						<Center inset={0} pos="absolute">
							<Loader aria-label={`Loading ${interval} candle history`} />
						</Center>
					) : null}
					{isLoadingMore ? (
						<Text c="dimmed" size="xs" ta="center">
							Loading older candles…
						</Text>
					) : null}
				</Box>
			</Stack>
		</Paper>
	);
});
