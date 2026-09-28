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
import {
	CandlestickSeries,
	type CandlestickSeriesPartialOptions,
	type CreatePriceLineOptions,
	type DeepPartial,
	HistogramSeries,
	LineSeries,
	type LineSeriesPartialOptions,
	type Time,
	type TimeChartOptions,
} from "lightweight-charts";
import {
	memo,
	useCallback,
	useEffect,
	useMemo,
	useState,
	useSyncExternalStore,
} from "react";
import { ChartCanvas, PriceLine, Series } from "@/components/lightweight-chart";
import {
	barSpacing,
	chartOptions as baseChartOptions,
	candlePriceScaleOptions,
	candleSeriesOptions,
	chartHeight,
	chartIntervalOptions,
	defaultChartInterval,
	indicatorPriceLineOptions,
	indicatorPriceScaleOptions,
	indicatorSeriesOptions,
	loadOlderThreshold,
	minMaxPriceLineOptions,
	minVisibleBars,
	paneStretchFactors,
	volumeColors,
	volumePriceScaleOptions,
	volumeSeriesOptions,
} from "@/components/price-history-chart/config";
import { LiveStatus } from "@/components/price-history-chart/live-status";

export type {
	ChartIndicatorOptions,
	ChartIntervalOption,
	ChartReadoutOptions,
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/components/price-history-chart/types";

import type {
	ChartCandle,
	ChartInterval,
	PriceHistoryChartProps,
} from "@/components/price-history-chart/types";
import { useChartViewport } from "@/components/price-history-chart/use-chart-viewport";
import {
	chartPriceResolution,
	createCandlestickData,
	createIndicatorData,
	createVolumeData,
	formatChartTime,
	formatOhlc,
	formatPrice,
	getVisibleMinMax,
	isChartCandle,
} from "@/components/price-history-chart/utils";

export const PriceHistoryChart = memo(function PriceHistoryChart({
	enabled,
	indicator,
	extraReadout,
	paperPadding,
	source,
	symbol,
}: PriceHistoryChartProps) {
	const [interval, setInterval] = useState<ChartInterval>(defaultChartInterval);
	const state = useSyncExternalStore(
		source.subscribe,
		() => source.getSnapshot(interval),
		() => source.getSnapshot(interval),
	);
	useEffect(() => {
		if (!enabled) return;
		source.start();
		return () => source.stop();
	}, [enabled, source]);
	const { candles, hasMore, isLoading, isLoadingMore } = state;
	const colorScheme = useComputedColorScheme("dark");
	const data = useMemo(
		() => createCandlestickData(candles, interval),
		[candles, interval],
	);
	const indicatorData = useMemo(
		() => (indicator ? createIndicatorData(data, state.indicator) : []),
		[data, indicator, state.indicator],
	);
	const last = candles.at(-1) ?? null;
	const [active, setActive] = useState<ChartCandle | null>(null);
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
	const volumeData = useMemo(
		() => createVolumeData(data, candles, volumeColors),
		[candles, data],
	);
	const candleOptions = useMemo<CandlestickSeriesPartialOptions>(() => {
		const { base, fractionDigits, minMove } = chartPriceResolution(data);
		return {
			...candleSeriesOptions,
			priceFormat: {
				base,
				// Axis ticks carry floating-point noise near zero; round it at the
				// chart resolution so it cannot widen the price scale.
				formatter: (value: number) => formatPrice(value, fractionDigits),
				minMove,
				type: "custom",
			},
		};
	}, [data]);
	const indicatorOptions = useMemo<LineSeriesPartialOptions | undefined>(
		() =>
			indicator && {
				...indicatorSeriesOptions,
				autoscaleInfoProvider: () => ({
					priceRange: {
						maxValue: indicator.bounds.max,
						minValue: indicator.bounds.min,
					},
				}),
				priceFormat: {
					formatter: indicator.formatValue,
					minMove: indicator.minMove,
					type: "custom",
				},
			},
		[indicator],
	);
	const indicatorPriceLines = useMemo<readonly CreatePriceLineOptions[]>(
		() =>
			(indicator?.lines ?? []).map(({ price, title }) => ({
				...indicatorPriceLineOptions[colorScheme],
				price,
				title,
			})),
		[colorScheme, indicator],
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
	const minMax = useMemo(
		() => getVisibleMinMax(data, visibleRange),
		[data, visibleRange],
	);
	const handleCrosshairMove = useCallback((value: unknown) => {
		setActive(isChartCandle(value) ? value : null);
	}, []);

	const readout = active ?? last;
	const readoutTime =
		active?.time ??
		(last ? Math.floor(Date.parse(last.open_time) / 1_000) : null);
	return (
		<Paper component="section" p={paperPadding}>
			<Stack gap="md">
				<SegmentedControl
					data={chartIntervalOptions.map(({ label, value }) => ({
						label,
						value,
					}))}
					fullWidth
					onChange={(value) => {
						setActive(null);
						setInterval(value as ChartInterval);
					}}
					value={interval}
				/>
				<Box pos="relative">
					<LiveStatus
						connection={state.connection}
						error={state.error}
						freshness={state.freshness}
					/>
					{readout && readoutTime !== null ? (
						<Text aria-live="polite" ff="monospace" mb="xs" size="sm">
							{formatChartTime(readoutTime, interval)} · {formatOhlc(readout)}
							{extraReadout ? (
								<>
									{" "}
									·{" "}
									<Text c="blue.4" component="span" fw={700} inherit>
										{extraReadout.label} {extraReadout.format(readout)}
									</Text>
								</>
							) : null}
						</Text>
					) : null}
					{last === null && !isLoading ? (
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
						<Series
							data={volumeData}
							definition={HistogramSeries}
							onBeforeDataChange={onBeforeDataChange}
							options={volumeSeriesOptions}
							priceScale={volumePriceScaleOptions}
						/>
						<Series
							data={data}
							definition={CandlestickSeries}
							onCrosshairMove={handleCrosshairMove}
							options={candleOptions}
							priceScale={candlePriceScaleOptions}
						>
							{minMax && (
								<>
									<PriceLine
										options={{
											...minMaxPriceLineOptions[colorScheme],
											price: minMax.min,
											title: minMax.min === minMax.max ? "Min / Max" : "Min",
										}}
									/>
									{minMax.min !== minMax.max && (
										<PriceLine
											options={{
												...minMaxPriceLineOptions[colorScheme],
												price: minMax.max,
												title: "Max",
											}}
										/>
									)}
								</>
							)}
						</Series>
						{indicatorOptions ? (
							<Series
								data={indicatorData}
								definition={LineSeries}
								options={indicatorOptions}
								pane={1}
								priceScale={indicatorPriceScaleOptions}
							>
								{indicatorPriceLines.map((options) => (
									<PriceLine key={options.price} options={options} />
								))}
							</Series>
						) : null}
					</ChartCanvas>
					{isLoading && last === null ? (
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
