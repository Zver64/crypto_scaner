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
	memo,
	useCallback,
	useEffect,
	useMemo,
	useState,
	useSyncExternalStore,
} from "react";
import {
	CandlestickSeries,
	type ChartCandlestick,
	type ChartCandlestickOptions,
	ChartCanvas,
	type ChartCanvasOptions,
	type ChartLineOptions,
	type ChartPriceLineOptions,
	chartPriceResolution,
	LineSeries,
	PriceLine,
} from "@/components/lightweight-chart";
import { LiveStatus } from "@/components/price-history-chart/live-status";

export type {
	ChartIndicatorOptions,
	ChartIntervalOption,
	ChartReadoutOptions,
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/components/price-history-chart/types";

import type {
	ChartInterval,
	PriceHistoryChartProps,
} from "@/components/price-history-chart/types";
import { useChartViewport } from "@/components/price-history-chart/use-chart-viewport";
import {
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

const leftLoadThreshold = 10;
const candleWidth = 7.5;
const paneStretchFactors = [3, 1] as const;

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
		source.start(intervals[0].value);
		return () => source.stop();
	}, [enabled, source, intervals[0].value]);
	useEffect(() => {
		if (enabled) source.select(interval);
	}, [enabled, source, interval]);
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
	const [active, setActive] = useState<ChartCandlestick | null>(null);
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
	const chartOptions = useMemo<ChartCanvasOptions>(
		() => ({
			background: colors.background,
			text: colors.text,
			grid: colors.grid,
			barSpacing: candleWidth,
			minBarSpacing: 2,
			timeFormatter: (time) => formatTime(time, interval),
			timeVisible:
				intervals.find((item) => item.value === interval)?.showTime ?? false,
		}),
		[colors, formatTime, interval, intervals],
	);
	const candleOptions = useMemo<ChartCandlestickOptions>(
		() => ({
			downColor: colors.down,
			formatPrice,
			...chartPriceResolution(data),
			upColor: colors.up,
		}),
		[colors, data],
	);
	const indicatorOptions = useMemo<ChartLineOptions | undefined>(
		() =>
			indicator && {
				bounds: indicator.bounds,
				color: theme.colors.blue[5],
				formatValue: indicator.formatValue,
				minMove: indicator.minMove,
			},
		[indicator, theme],
	);
	const indicatorPriceLines = useMemo<readonly ChartPriceLineOptions[]>(
		() =>
			(indicator?.lines ?? []).map(({ price, title }) => ({
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
		barWidth: candleWidth,
		data,
		hasMore,
		isLoadingMore,
		minVisibleBars: 24,
		onLoadOlder,
		threshold: leftLoadThreshold,
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
						paneStretchFactors={indicator ? paneStretchFactors : []}
						ref={chartRef}
						role="img"
						style={{ height: 440, width: "100%" }}
					>
						<CandlestickSeries
							data={data}
							onBeforeDataChange={onBeforeDataChange}
							onCrosshairMove={handleCrosshairMove}
							options={candleOptions}
						>
							{minMax && (
								<>
									<PriceLine
										options={{
											price: minMax.min,
											title: minMax.min === minMax.max ? "Min / Max" : "Min",
											color: colors.text,
											lineVisible: false,
										}}
									/>
									{minMax.min !== minMax.max && (
										<PriceLine
											options={{
												price: minMax.max,
												title: "Max",
												color: colors.text,
												lineVisible: false,
											}}
										/>
									)}
								</>
							)}
						</CandlestickSeries>
						{indicatorOptions ? (
							<LineSeries
								data={indicatorData}
								options={indicatorOptions}
								pane={1}
							>
								{indicatorPriceLines.map((options) => (
									<PriceLine key={options.price} options={options} />
								))}
							</LineSeries>
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
