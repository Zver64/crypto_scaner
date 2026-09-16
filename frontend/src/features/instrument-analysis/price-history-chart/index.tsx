import {
	Box,
	Center,
	Loader,
	Text,
	useComputedColorScheme,
	useMantineTheme,
} from "@mantine/core";
import {
	CandlestickSeries,
	ColorType,
	createChart,
	type DeepPartial,
	type IChartApi,
	type ISeriesApi,
	type LogicalRange,
	type Time,
	type TimeChartOptions,
} from "lightweight-charts";
import { useEffect, useMemo, useRef, useState } from "react";
import type { CandleInterval, PriceCandle } from "@/api/candle-history";
import {
	type ChartCandle,
	chartPriceResolution,
	createCandlestickData,
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
	onLoadOlder(): void;
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

export function InstrumentPriceHistoryChart({
	candles,
	hasMore,
	interval,
	isLoading,
	isLoadingMore,
	onLoadOlder,
	symbol,
}: InstrumentPriceHistoryChartProps) {
	const containerRef = useRef<HTMLDivElement>(null);
	const chartRef = useRef<IChartApi | null>(null);
	const seriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
	const previousLengthRef = useRef(0);
	const loadStateRef = useRef({ hasMore, isLoadingMore, onLoadOlder });
	const theme = useMantineTheme();
	const colorScheme = useComputedColorScheme("dark");
	const data = useMemo(
		() => createCandlestickData(candles, interval),
		[candles, interval],
	);
	const last = candles.at(-1) ?? null;
	const [active, setActive] = useState<ChartCandle | null>(null);

	loadStateRef.current = { hasMore, isLoadingMore, onLoadOlder };

	useEffect(() => {
		const container = containerRef.current;
		if (!container) return;
		const colors: ChartColors = {
			background: colorScheme === "dark" ? theme.colors.dark[7] : theme.white,
			down: theme.colors.red[6],
			grid:
				colorScheme === "dark" ? theme.colors.dark[5] : theme.colors.gray[3],
			text:
				colorScheme === "dark" ? theme.colors.dark[0] : theme.colors.gray[7],
			up: theme.colors.green[6],
		};
		const options: DeepPartial<TimeChartOptions> = {
			autoSize: true,
			height: 300,
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
		};
		const chart = createChart(container, options);
		const series = chart.addSeries(CandlestickSeries, {
			borderVisible: false,
			downColor: colors.down,
			priceFormat: {
				base: 100,
				formatter: formatPrice,
				minMove: 0.01,
				type: "custom",
			},
			upColor: colors.up,
			wickDownColor: colors.down,
			wickUpColor: colors.up,
		});
		chartRef.current = chart;
		seriesRef.current = series;
		previousLengthRef.current = 0;
		const handleCrosshairMove: Parameters<
			typeof chart.subscribeCrosshairMove
		>[0] = (parameter) => {
			const value = parameter.seriesData.get(series);
			setActive(isChartCandle(value) ? value : null);
		};
		const handleRange = (range: LogicalRange | null) => {
			const state = loadStateRef.current;
			if (
				range !== null &&
				range.from < leftLoadThreshold &&
				state.hasMore &&
				!state.isLoadingMore
			) {
				state.onLoadOlder();
			}
		};
		chart.subscribeCrosshairMove(handleCrosshairMove);
		chart.timeScale().subscribeVisibleLogicalRangeChange(handleRange);
		return () => {
			chart.unsubscribeCrosshairMove(handleCrosshairMove);
			chart.timeScale().unsubscribeVisibleLogicalRangeChange(handleRange);
			chartRef.current = null;
			seriesRef.current = null;
			chart.remove();
		};
	}, [colorScheme, interval, theme]);

	useEffect(() => {
		const chart = chartRef.current;
		const series = seriesRef.current;
		const container = containerRef.current;
		if (!chart || !series || !container || data.length === 0) return;
		const previousLength = previousLengthRef.current;
		const previousRange = chart.timeScale().getVisibleLogicalRange();
		series.applyOptions({
			priceFormat: {
				formatter: formatPrice,
				...chartPriceResolution(data),
				type: "custom",
			},
		});
		series.setData(data);
		if (previousLength === 0) {
			const visibleBars = Math.max(
				24,
				Math.floor(container.clientWidth / candleWidth),
			);
			chart.timeScale().setVisibleLogicalRange({
				from: Math.max(0, data.length - visibleBars),
				to: data.length - 1,
			});
		} else if (previousRange && data.length > previousLength) {
			const prepended = data.length - previousLength;
			chart.timeScale().setVisibleLogicalRange({
				from: previousRange.from + prepended,
				to: previousRange.to + prepended,
			});
		}
		previousLengthRef.current = data.length;
	}, [data]);

	const readout = active ?? last;
	const readoutTime =
		active?.time ??
		(last ? Math.floor(Date.parse(last.open_time) / 1_000) : null);
	return (
		<Box pos="relative">
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
			<div
				aria-label={`${symbol}: ${interval} closed candlestick history. ${candles.length} candles loaded.${hasMore ? " Scroll left to load older candles." : " Earliest stored candle reached."}`}
				ref={containerRef}
				role="img"
				style={{ height: 300, width: "100%" }}
			/>
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
