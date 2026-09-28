import {
	Box,
	Center,
	Loader,
	Paper,
	SegmentedControl,
	Stack,
	Text,
	useComputedColorScheme,
	useMantineTheme,
} from "@mantine/core";
import {
	CandlestickSeries,
	type CandlestickSeriesPartialOptions,
	type CreatePriceLineOptions,
	type DeepPartial,
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
import { priceHistoryChartConfig as config } from "@/components/price-history-chart/config";
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
	formatOhlc,
	formatPrice,
	getVisibleMinMax,
	isChartCandle,
} from "@/components/price-history-chart/utils";

interface ChartColors {
	background: string;
	down: string;
	grid: string;
	text: string;
	up: string;
}

export const PriceHistoryChart = memo(function PriceHistoryChart({
	enabled,
	indicator,
	intervals,
	formatTime,
	nextOpen,
	extraReadout,
	paperPadding,
	source,
	symbol,
}: PriceHistoryChartProps) {
	const [interval, setInterval] = useState<ChartInterval>(intervals[0].value);
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
	const theme = useMantineTheme();
	const colorScheme = useComputedColorScheme("dark");
	const data = useMemo(
		() => createCandlestickData(candles, interval, nextOpen),
		[candles, interval, nextOpen],
	);
	const indicatorData = useMemo(
		() => (indicator ? createIndicatorData(data, state.indicator) : []),
		[data, indicator, state.indicator],
	);
	const last = candles.at(-1) ?? null;
	const [active, setActive] = useState<ChartCandle | null>(null);
	const colors = useMemo<ChartColors>(
		() => ({
			background: colorScheme === "dark" ? theme.colors.dark[7] : theme.white,
			down: theme.colors.red[6],
			grid:
				colorScheme === "dark" ? theme.colors.dark[5] : theme.colors.gray[3],
			text:
				colorScheme === "dark" ? theme.colors.dark[0] : theme.colors.gray[7],
			up: theme.colors.green[6],
		}),
		[colorScheme, theme],
	);
	const chartOptions = useMemo<DeepPartial<TimeChartOptions>>(
		() => ({
			...config.chart,
			grid: {
				horzLines: { color: colors.grid },
				vertLines: { color: colors.grid },
			},
			layout: {
				background: { color: colors.background },
				textColor: colors.text,
			},
			localization: {
				timeFormatter: (time: Time) =>
					typeof time === "number" ? formatTime(time, interval) : String(time),
			},
			rightPriceScale: { borderColor: colors.grid },
			timeScale: {
				...config.chart.timeScale,
				borderColor: colors.grid,
				timeVisible:
					intervals.find((item) => item.value === interval)?.showTime ?? false,
			},
		}),
		[colors, formatTime, interval, intervals],
	);
	const candleOptions = useMemo<CandlestickSeriesPartialOptions>(() => {
		const { base, fractionDigits, minMove } = chartPriceResolution(data);
		return {
			...config.candles.series,
			downColor: colors.down,
			upColor: colors.up,
			wickDownColor: colors.down,
			wickUpColor: colors.up,
			priceFormat: {
				base,
				// Axis ticks carry floating-point noise near zero; round it at the
				// chart resolution so it cannot widen the price scale.
				formatter: (value: number) => formatPrice(value, fractionDigits),
				minMove,
				type: "custom",
			},
		};
	}, [colors, data]);
	const indicatorOptions = useMemo<LineSeriesPartialOptions | undefined>(
		() =>
			indicator && {
				...config.indicator.series,
				autoscaleInfoProvider: () => ({
					priceRange: {
						maxValue: indicator.bounds.max,
						minValue: indicator.bounds.min,
					},
				}),
				color: theme.colors.blue[5],
				priceFormat: {
					formatter: indicator.formatValue,
					minMove: indicator.minMove,
					type: "custom",
				},
			},
		[indicator, theme],
	);
	const indicatorPriceLines = useMemo<readonly CreatePriceLineOptions[]>(
		() =>
			(indicator?.lines ?? []).map(({ price, title }) => ({
				...config.priceLine,
				color: colors.grid,
				price,
				title,
			})),
		[colors.grid, indicator],
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
		barWidth: config.chart.timeScale.barSpacing,
		data,
		hasMore,
		isLoadingMore,
		minVisibleBars: config.viewport.minVisibleBars,
		onLoadOlder,
		threshold: config.viewport.loadOlderThreshold,
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
					data={intervals.map(({ label, value }) => ({ label, value }))}
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
							{formatTime(readoutTime, interval)} · {formatOhlc(readout)}
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
						paneStretchFactors={indicator ? config.paneStretchFactors : []}
						ref={chartRef}
						role="img"
						style={{ height: config.height, width: "100%" }}
					>
						<Series
							data={data}
							definition={CandlestickSeries}
							onBeforeDataChange={onBeforeDataChange}
							onCrosshairMove={handleCrosshairMove}
							options={candleOptions}
							priceScale={config.candles.priceScale}
						>
							{minMax && (
								<>
									<PriceLine
										options={{
											...config.priceLine,
											price: minMax.min,
											title: minMax.min === minMax.max ? "Min / Max" : "Min",
											color: colors.text,
											lineVisible: false,
										}}
									/>
									{minMax.min !== minMax.max && (
										<PriceLine
											options={{
												...config.priceLine,
												price: minMax.max,
												title: "Max",
												color: colors.text,
												lineVisible: false,
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
								priceScale={config.indicator.priceScale}
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
