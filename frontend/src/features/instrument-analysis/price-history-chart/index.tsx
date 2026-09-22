import {
	Box,
	Center,
	Loader,
	Text,
	useComputedColorScheme,
	useMantineTheme,
} from "@mantine/core";
import {
	ColorType,
	type CreatePriceLineOptions,
	type DeepPartial,
	type IChartApi,
	LineStyle,
	type LogicalRange,
	type PriceScaleOptions,
	type Time,
	type TimeChartOptions,
} from "lightweight-charts";
import { useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";
import type {
	CandleInterval,
	IndicatorPoint,
	LiveCandleServerMessageFreshness,
} from "@/api/generated/models";
import {
	CandlestickSeries,
	LightweightChart,
	type LightweightChartHandle,
	LineSeries,
	PriceLine,
} from "@/components/lightweight-chart";
import type { PriceCandle } from "@/features/instrument-analysis/candle-page";
import { LiveStatus } from "@/features/instrument-analysis/price-history-chart/live-status";
import {
	type ChartCandle,
	chartPriceResolution,
	createCandlestickData,
	createRsiData,
	formatCandleRange,
	formatOhlc,
	formatPrice,
	formatUtcTimestamp,
	isChartCandle,
} from "@/features/instrument-analysis/price-history-chart/utils";

interface InstrumentPriceHistoryChartProps {
	candles: readonly PriceCandle[];
	hasMore: boolean;
	interval: CandleInterval;
	isLoading: boolean;
	isLoadingMore: boolean;
	liveConnection: "connecting" | "connected" | "disconnected";
	liveFreshness: LiveCandleServerMessageFreshness;
	liveError?: string;
	onLoadOlder(): void;
	rsi: readonly IndicatorPoint[];
	symbol: string;
}

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

export function InstrumentPriceHistoryChart({
	candles,
	hasMore,
	interval,
	isLoading,
	isLoadingMore,
	liveConnection,
	liveFreshness,
	liveError,
	onLoadOlder,
	rsi,
	symbol,
}: InstrumentPriceHistoryChartProps) {
	const chartRef = useRef<LightweightChartHandle>(null);
	const chartInstanceRef = useRef<IChartApi | null>(null);
	const previousLengthRef = useRef(0);
	const previousFirstTimeRef = useRef<Time | undefined>(undefined);
	const previousRangeRef = useRef<LogicalRange | null>(null);
	const theme = useMantineTheme();
	const colorScheme = useComputedColorScheme("dark");
	const data = useMemo(
		() => createCandlestickData(candles, interval),
		[candles, interval],
	);
	const rsiData = useMemo(
		() => createRsiData(candles, rsi, interval),
		[candles, interval, rsi],
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
			layout: {
				background: { color: colors.background, type: ColorType.Solid },
				textColor: colors.text,
			},
			grid: {
				horzLines: { color: colors.grid },
				vertLines: { color: colors.grid },
			},
			localization: {
				timeFormatter: (time: Time) =>
					typeof time === "number"
						? formatUtcTimestamp(time, interval)
						: String(time),
			},
			rightPriceScale: { borderColor: colors.grid },
			timeScale: {
				barSpacing: candleWidth,
				borderColor: colors.grid,
				minBarSpacing: 2,
				secondsVisible: false,
				timeVisible: interval === "1h",
			},
		}),
		[colors, interval],
	);
	const candleOptions = useMemo(
		() => ({
			borderVisible: false,
			downColor: colors.down,
			priceFormat: {
				formatter: formatPrice,
				...chartPriceResolution(data),
				type: "custom" as const,
			},
			upColor: colors.up,
			wickDownColor: colors.down,
			wickUpColor: colors.up,
		}),
		[colors, data],
	);
	const rsiOptions = useMemo(
		() => ({
			autoscaleInfoProvider: () => ({
				priceRange: { maxValue: 100, minValue: 0 },
			}),
			color: theme.colors.blue[5],
			lastValueVisible: true,
			lineWidth: 2 as const,
			priceFormat: {
				formatter: (value: number) => value.toFixed(1),
				minMove: 0.1,
				type: "custom" as const,
			},
			priceLineVisible: false,
		}),
		[theme],
	);
	const rsiPriceScaleOptions = useMemo<DeepPartial<PriceScaleOptions>>(
		() => ({
			autoScale: true,
			scaleMargins: { bottom: 0, top: 0 },
		}),
		[],
	);
	const rsiPriceLines = useMemo<readonly CreatePriceLineOptions[]>(
		() =>
			[30, 70].map((price) => ({
				axisLabelVisible: true,
				color: colors.grid,
				lineStyle: LineStyle.Dashed,
				lineWidth: 1,
				price,
				title: `RSI ${price}`,
			})),
		[colors.grid],
	);

	const handleBeforeCandleDataChange = useCallback(() => {
		const chartHandle = chartRef.current;
		previousRangeRef.current =
			chartHandle === null || previousLengthRef.current === 0
				? null
				: chartHandle.api().timeScale().getVisibleLogicalRange();
	}, []);
	const handleCrosshairMove = useCallback((value: unknown) => {
		setActive(isChartCandle(value) ? value : null);
	}, []);
	const handleRangeChange = useCallback(
		(range: LogicalRange | null) => {
			if (
				range !== null &&
				range.from < leftLoadThreshold &&
				hasMore &&
				!isLoadingMore
			) {
				onLoadOlder();
			}
		},
		[hasMore, isLoadingMore, onLoadOlder],
	);

	useLayoutEffect(() => {
		const chartHandle = chartRef.current;
		if (chartHandle === null || data.length === 0) {
			if (data.length === 0) {
				previousLengthRef.current = 0;
				previousFirstTimeRef.current = undefined;
				previousRangeRef.current = null;
			}
			return;
		}
		const chart = chartHandle.api();
		if (chartInstanceRef.current !== chart) {
			chartInstanceRef.current = chart;
			previousLengthRef.current = 0;
			previousFirstTimeRef.current = undefined;
			previousRangeRef.current = null;
		}
		const previousLength = previousLengthRef.current;
		const previousRange = previousRangeRef.current;
		previousRangeRef.current = null;
		if (previousLength === 0) {
			const visibleBars = Math.max(
				24,
				Math.floor(chartHandle.containerWidth() / candleWidth),
			);
			chart.timeScale().setVisibleLogicalRange({
				from: Math.max(0, data.length - visibleBars),
				to: data.length - 1,
			});
		} else if (previousRange && data.length > previousLength) {
			const previousFirst = previousFirstTimeRef.current;
			const prepended =
				previousFirst === undefined
					? 0
					: Math.max(
							0,
							data.findIndex((item) => item.time === previousFirst),
						);
			const appended = Math.max(0, data.length - previousLength - prepended);
			const followingLatest = previousRange.to >= previousLength - 1.5;
			const shift = prepended + (followingLatest ? appended : 0);
			chart.timeScale().setVisibleLogicalRange({
				from: previousRange.from + shift,
				to: previousRange.to + shift,
			});
		}
		previousLengthRef.current = data.length;
		previousFirstTimeRef.current = data[0]?.time;
	}, [data]);

	const readout = active ?? last;
	const readoutTime =
		active?.time ??
		(last ? Math.floor(Date.parse(last.open_time) / 1_000) : null);
	return (
		<Box pos="relative">
			<LiveStatus
				connection={liveConnection}
				error={liveError}
				freshness={liveFreshness}
			/>
			{readout && readoutTime !== null ? (
				<Text aria-live="polite" ff="monospace" mb="xs" size="sm">
					{formatUtcTimestamp(readoutTime, interval)} · {formatOhlc(readout)} ·{" "}
					<Text c="blue.4" component="span" fw={700} inherit>
						R {formatCandleRange(readout)}
					</Text>
				</Text>
			) : null}
			{last === null && !isLoading ? (
				<Text c="dimmed">No closed candles are available.</Text>
			) : null}
			<LightweightChart
				aria-label={`${symbol}: ${interval} candlestick history with the current live candle. ${candles.length} candles loaded.${hasMore ? " Scroll left to load older candles." : " Earliest stored candle reached."}`}
				onVisibleLogicalRangeChange={handleRangeChange}
				options={chartOptions}
				paneStretchFactors={paneStretchFactors}
				ref={chartRef}
				role="img"
				style={{ height: 440, width: "100%" }}
			>
				<CandlestickSeries
					data={data}
					onBeforeDataChange={handleBeforeCandleDataChange}
					onCrosshairMove={handleCrosshairMove}
					options={candleOptions}
				/>
				<LineSeries
					data={rsiData}
					options={rsiOptions}
					pane={1}
					priceScaleOptions={rsiPriceScaleOptions}
				>
					{rsiPriceLines.map((options) => (
						<PriceLine key={options.price} options={options} />
					))}
				</LineSeries>
			</LightweightChart>
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
	);
}
