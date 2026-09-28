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
	chartHeight,
	chartIntervalOptions,
	defaultChartInterval,
	loadOlderThreshold,
	minVisibleBars,
	paneStretchFactors,
} from "@/components/price-history-chart/config";
import { IndicatorSeries } from "@/components/price-history-chart/indicator-series";
import { LiveStatus } from "@/components/price-history-chart/live-status";
import type {
	ChartCandle,
	ChartInterval,
	PriceHistoryChartProps,
} from "@/components/price-history-chart/types";
import { useChartViewport } from "@/components/price-history-chart/use-chart-viewport";
import { usePriceHistory } from "@/components/price-history-chart/use-price-history";
import {
	createCandlestickData,
	createIndicatorData,
	formatChartTime,
} from "@/components/price-history-chart/utils";
import { VolumeSeries } from "@/components/price-history-chart/volume-series";

export type {
	ChartIndicatorOptions,
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
	indicator,
	extraReadout,
	paperPadding,
	source,
	symbol,
}: PriceHistoryChartProps) {
	const [interval, setInterval] = useState<ChartInterval>(defaultChartInterval);
	const [activeCandle, setActiveCandle] = useState<ChartCandle | null>(null);
	const colorScheme = useComputedColorScheme("dark");
	const {
		candles,
		connection,
		error,
		freshness,
		hasMore,
		indicator: indicatorPoints,
		isLoading,
		isLoadingMore,
	} = usePriceHistory(source, interval, enabled);

	const data = useMemo(
		() => createCandlestickData(candles, interval),
		[candles, interval],
	);
	const indicatorData = useMemo(
		() => (indicator ? createIndicatorData(data, indicatorPoints) : []),
		[data, indicator, indicatorPoints],
	);
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
					{!hasCandles && !isLoading ? (
						<Text c="dimmed">No closed candles are available.</Text>
					) : null}
					<ChartCanvas
						key={`${symbol}:${interval}`}
						aria-label={`${symbol}: ${interval} candlestick history with the current live candle. ${candles.length} candles loaded.${hasMore ? " Scroll left to load older candles." : " Earliest stored candle reached."}`}
						onVisibleLogicalRangeChange={onVisibleLogicalRangeChange}
						options={chartOptions}
						paneStretchFactors={indicator ? paneStretchFactors : []}
						ref={chartRef}
						role="img"
						style={{ height: chartHeight, width: "100%" }}
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
							visibleRange={visibleRange}
						/>
						{indicator ? (
							<IndicatorSeries data={indicatorData} indicator={indicator} />
						) : null}
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
