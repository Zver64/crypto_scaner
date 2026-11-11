import {
	Box,
	Center,
	Loader,
	Paper,
	SegmentedControl,
	Stack,
	Text,
	useComputedColorScheme,
	useMatches,
} from "@mantine/core";
import { type CSSProperties, useState } from "react";

import { ChartCanvas } from "@/components/lightweight-chart";
import { CandleSeries } from "@/features/candle-chart/candle-series";
import { timeChartOptions } from "@/features/candle-chart/chart-options";
import { ChartReadout } from "@/features/candle-chart/chart-readout";
import {
	barSpacing,
	candlePaneHeight,
	chartIntervalOptions,
	compactCandlePaneShare,
	compactIndicatorPaneHeight,
	defaultChartInterval,
	indicatorPaneHeight,
	loadOlderThreshold,
	minVisibleBars,
} from "@/features/candle-chart/config";
import { emptyChartLine } from "@/features/candle-chart/constants";
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
import { usePriceHistory } from "@/features/candle-chart/use-price-history";
import { createIndicatorLegend } from "@/features/candle-chart/utils";
import { VolumeSeries } from "@/features/candle-chart/volume-series";

export { createChartPriceLevels } from "@/features/candle-chart/utils";
export type {
	ChartIndicatorLayout,
	ChartIntervalOption,
	ChartPriceLevels,
	ChartReadoutOptions,
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/features/candle-chart/types";

// The switch shows the short interval codes to stay compact.
const intervalControlData = chartIntervalOptions.map(({ value }) => ({
	label: value,
	value,
}));

// Sources deliver chart-ready data (see PriceHistorySnapshot.series) and the
// chart wrappers compare what they receive, so the chart transforms nothing
// while rendering and needs no memoization.
export function PriceHistoryChart({
	enabled,
	indicators: layouts,
	extraReadout,
	fillHeight = false,
	intervals,
	live = true,
	paperPadding,
	priceLevels,
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
	const { indicators, panes } = layouts[interval];
	const colorScheme = useComputedColorScheme("dark");
	// Phones give the chart the full card width and size it from the screen.
	// The height is read once, so an opening keyboard does not resize the chart.
	const compact = useMatches(
		{ base: true, sm: false },
		{ getInitialValueInEffect: false },
	);
	const [screenHeight] = useState(() => window.innerHeight);
	const {
		candles,
		connection,
		error,
		freshness,
		hasMore,
		isLoading,
		isLoadingMore,
		markers,
		series,
	} = usePriceHistory(source, interval, enabled);
	const line = (key: string) => series.lines[key] ?? emptyChartLine;
	const overlays = indicators.filter(
		(indicator) => indicator.placement === "overlay",
	);

	const {
		chartRef,
		onBeforeDataChange,
		onVisibleLogicalRangeChange,
		visibleRange,
	} = useChartViewport({
		barWidth: barSpacing,
		data: series.candles,
		hasMore,
		isLoadingMore,
		minVisibleBars,
		onLoadOlder: () => source.loadOlder(interval),
		threshold: loadOlderThreshold,
	});

	const selectInterval = (value: ChartInterval) => {
		setActiveCandle(null);
		setInterval(value);
	};
	const readoutCandle = activeCandle ?? candles.at(-1);
	const legendSlotIndex = activeCandle
		? series.candles.findIndex((slot) => slot.time === activeCandle.time)
		: series.candles.length - 1;
	const legendItems = createIndicatorLegend(
		indicators,
		series.lines,
		legendSlotIndex,
	);
	const hasCandles = candles.length > 0;
	const candleHeight = compact
		? Math.max(
				candlePaneHeight,
				Math.round(screenHeight * compactCandlePaneShare),
			)
		: candlePaneHeight;
	const paneHeight = compact ? compactIndicatorPaneHeight : indicatorPaneHeight;
	const chartHeight = candleHeight + panes.length * paneHeight;
	const fillStyle: CSSProperties | undefined = fillHeight
		? { display: "flex", flex: 1, flexDirection: "column" }
		: undefined;

	return (
		<Paper
			component="section"
			px={compact ? 0 : paperPadding}
			py={paperPadding}
			style={fillStyle}
		>
			<Stack gap="xs" style={fillStyle}>
				<SegmentedControl<ChartInterval>
					data={intervalOptions}
					mx={compact ? paperPadding : 0}
					onChange={selectInterval}
					size="xs"
					style={{ alignSelf: "flex-start" }}
					value={interval}
				/>
				<Box pos="relative" style={fillStyle}>
					{!hasCandles && !isLoading ? (
						<Text c="dimmed">No closed candles are available.</Text>
					) : null}
					<ChartCanvas
						key={`${symbol}:${interval}`}
						aria-label={`${symbol}: ${interval} candlestick history${live ? " with the current live candle" : " of the completed backtest"}. ${candles.length} candles loaded.${hasMore ? " Scroll left to load older candles." : " Earliest stored candle reached."}`}
						onVisibleLogicalRangeChange={onVisibleLogicalRangeChange}
						options={timeChartOptions[colorScheme][interval]}
						paneStretchFactors={[candleHeight, ...panes.map(() => paneHeight)]}
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
							data={series.volumes}
							onBeforeDataChange={onBeforeDataChange}
						/>
						<CandleSeries
							currentPrice={candles.at(-1)?.close}
							data={series.candles}
							markers={markers}
							onActiveCandleChange={setActiveCandle}
							priceLevels={priceLevels}
							priceResolution={series.priceResolution}
							visibleRange={visibleRange}
						/>
						{overlays.flatMap(({ id, lines }) =>
							lines.map((indicatorLine) => (
								<OverlayIndicatorSeries
									data={line(`${id}:${indicatorLine.output}`).data}
									key={`${id}:${indicatorLine.output}`}
									line={indicatorLine}
									priceResolution={series.priceResolution}
								/>
							)),
						)}
						{panes.flatMap(
							(
								{ autoscaleInfoProvider, indicators: paneIndicators, scale },
								paneIndex,
							) =>
								paneIndicators.flatMap(({ id, lines }, indicatorIndex) =>
									lines.map((indicatorLine, lineIndex) => (
										<PaneIndicatorSeries
											autoscaleInfoProvider={autoscaleInfoProvider}
											data={line(`${id}:${indicatorLine.output}`)}
											drawsLevels={indicatorIndex === 0 && lineIndex === 0}
											key={`${id}:${indicatorLine.output}`}
											line={indicatorLine}
											pane={paneIndex + 1}
											scale={scale}
										/>
									)),
								),
						)}
					</ChartCanvas>
					{/* The status, readout and legend lie over the chart, so they take
					no height of their own and let the pointer through. */}
					<Stack
						gap={2}
						left={0}
						maw="75%"
						pl={compact ? paperPadding : 0}
						pos="absolute"
						style={{ pointerEvents: "none", zIndex: 3 }}
						top={0}
					>
						{live ? (
							<LiveStatus
								connection={connection}
								error={error}
								freshness={freshness}
							/>
						) : null}
						{readoutCandle ? (
							<ChartReadout candle={readoutCandle} extra={extraReadout} />
						) : null}
						{readoutCandle ? <IndicatorLegend items={legendItems} /> : null}
					</Stack>
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
}
