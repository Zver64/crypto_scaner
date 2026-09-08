import {
	Box,
	Text,
	useComputedColorScheme,
	useMantineTheme,
} from "@mantine/core";
import {
	CandlestickSeries,
	ColorType,
	createChart,
	type DeepPartial,
	type Time,
	type TimeChartOptions,
} from "lightweight-charts";
import { useEffect, useMemo, useRef, useState } from "react";
import type { PriceCandle, PriceHistoryWindow } from "@/api/client";
import {
	availableCandles,
	type ChartCandle,
	type ChartCandleSlot,
	chartPriceResolution,
	createCandlestickData,
	formatOhlc,
	formatPrice,
	formatUtcTimestamp,
	isChartCandle,
} from "@/features/instrument-analysis/price-history-chart/utils";

interface InstrumentPriceHistoryChartProps {
	candles: readonly (PriceCandle | null)[];
	symbol: string;
	window: PriceHistoryWindow;
}

interface ChartColors {
	background: string;
	down: string;
	grid: string;
	text: string;
	up: string;
}

interface MountChartOptions {
	colors: ChartColors;
	container: HTMLElement;
	data: ChartCandleSlot[];
	onCrosshair(candle: ChartCandle | null): void;
}

export function mountCandlestickChart({
	colors,
	container,
	data,
	onCrosshair,
}: MountChartOptions): () => void {
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
				typeof time === "number" ? formatUtcTimestamp(time) : String(time),
		},
		rightPriceScale: { borderColor: colors.grid },
		timeScale: {
			borderColor: colors.grid,
			// Allow all 721 hourly slots to fit even on narrow Telegram screens.
			minBarSpacing: 0.1,
			secondsVisible: false,
			timeVisible: true,
		},
	};
	const chart = createChart(container, options);
	const series = chart.addSeries(CandlestickSeries, {
		borderVisible: false,
		downColor: colors.down,
		priceFormat: {
			formatter: formatPrice,
			...chartPriceResolution(data),
			type: "custom",
		},
		upColor: colors.up,
		wickDownColor: colors.down,
		wickUpColor: colors.up,
	});
	series.setData(data);
	const firstVisible = data[Math.max(0, data.length - 169)];
	const lastVisible = data.at(-1);
	if (firstVisible && lastVisible) {
		chart.timeScale().setVisibleRange({
			from: firstVisible.time,
			to: lastVisible.time,
		});
	}

	const handleCrosshairMove: Parameters<
		typeof chart.subscribeCrosshairMove
	>[0] = (param) => {
		const value = param.seriesData.get(series);
		onCrosshair(isChartCandle(value) ? value : null);
	};
	chart.subscribeCrosshairMove(handleCrosshairMove);

	return () => {
		chart.unsubscribeCrosshairMove(handleCrosshairMove);
		chart.remove();
	};
}

export function InstrumentPriceHistoryChart({
	candles,
	symbol,
	window,
}: InstrumentPriceHistoryChartProps) {
	const containerRef = useRef<HTMLDivElement>(null);
	const theme = useMantineTheme();
	const colorScheme = useComputedColorScheme("dark");
	const available = useMemo(() => availableCandles(candles), [candles]);
	const last = available.at(-1) ?? null;
	const [active, setActive] = useState<ChartCandle | null>(null);
	const data = useMemo(
		() => createCandlestickData(candles, window.from),
		[candles, window.from],
	);

	useEffect(() => {
		if (!containerRef.current || last === null) return;
		setActive(null);
		return mountCandlestickChart({
			colors: {
				background: colorScheme === "dark" ? theme.colors.dark[7] : theme.white,
				down: theme.colors.red[6],
				grid:
					colorScheme === "dark" ? theme.colors.dark[5] : theme.colors.gray[3],
				text:
					colorScheme === "dark" ? theme.colors.dark[0] : theme.colors.gray[7],
				up: theme.colors.green[6],
			},
			container: containerRef.current,
			data,
			onCrosshair: setActive,
		});
	}, [colorScheme, data, last, theme]);

	if (last === null) {
		return (
			<span
				role="img"
				aria-label={`${symbol}: No hourly candle history available in the last 30 days`}
			>
				—
			</span>
		);
	}

	const readout = active ?? last;
	const readoutTime =
		active?.time ?? Math.floor(Date.parse(last.open_time) / 1_000);
	return (
		<Box>
			<Text aria-live="polite" ff="monospace" mb="xs" size="sm">
				{formatUtcTimestamp(readoutTime)} · {formatOhlc(readout)}
			</Text>
			<div
				aria-label={`${symbol}: Up to 30 days of hourly candlestick prices. ${available.length} of ${candles.length} hourly slots contain candles. Candle hours ${window.from} to ${window.to}. The latest 7 days are visible initially.`}
				ref={containerRef}
				role="img"
				style={{ height: 300, width: "100%" }}
			/>
		</Box>
	);
}
