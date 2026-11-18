import {
	Box,
	Button,
	Center,
	Group,
	Loader,
	Paper,
	SegmentedControl,
	Stack,
	Text,
	useComputedColorScheme,
	useMatches,
} from "@mantine/core";
import { type CSSProperties, useState } from "react";

import type { CandleInterval, ChartPlacement } from "@/api/generated/models";
import { ChartCanvas } from "@/components/lightweight-chart";
import { AreaShapeSeries } from "@/features/candle-chart/area-shape-series";
import { CandleSeries } from "@/features/candle-chart/candle-series";
import { timeChartOptions } from "@/features/candle-chart/chart-options";
import { ChartReadout } from "@/features/candle-chart/chart-readout";
import { ComparisonSeries } from "@/features/candle-chart/comparison-series";
import {
	barSpacing,
	candlePaneHeight,
	compactCandlePaneShare,
	compactIndicatorPaneHeight,
	indicatorPaneHeight,
	loadOlderThreshold,
	minVisibleBars,
} from "@/features/candle-chart/config";
import { IndicatorLegend } from "@/features/candle-chart/indicator-legend";
import {
	chartIntervalOptions,
	defaultChartInterval,
} from "@/features/candle-chart/intervals";
import { LineShapeSeries } from "@/features/candle-chart/line-shape-series";
import { LiveStatus } from "@/features/candle-chart/live-status";
import { MarkersShapeSeries } from "@/features/candle-chart/markers-shape-series";
import { SegmentsShapeSeries } from "@/features/candle-chart/segments-shape-series";
import type {
	ChartCandle,
	PriceHistoryChartProps,
} from "@/features/candle-chart/types";
import { useChartViewport } from "@/features/candle-chart/use-chart-viewport";
import { usePriceComparison } from "@/features/candle-chart/use-price-comparison";
import { usePriceHistory } from "@/features/candle-chart/use-price-history";
import { useThemeColor } from "@/features/candle-chart/use-theme-color";
import {
	createCandleMarkers,
	createIndicatorLegend,
} from "@/features/candle-chart/utils";
import { VolumeSeries } from "@/features/candle-chart/volume-series";

export { createChartPriceLevels } from "@/features/candle-chart/utils";
export type {
	ChartPriceLevels,
	PriceComparison,
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/features/candle-chart/types";

// Sources deliver chart-ready data (see PriceHistorySnapshot.series) and the
// chart wrappers compare what they receive, so the chart transforms nothing
// while rendering and needs no memoization.
export function PriceHistoryChart({
	comparison,
	fillHeight = false,
	intervals,
	live = true,
	paperPadding,
	priceLevels,
	source,
	symbol,
}: PriceHistoryChartProps) {
	const [interval, setInterval] = useState<CandleInterval>(
		intervals?.[0] ?? defaultChartInterval,
	);
	const intervalOptions = intervals
		? chartIntervalOptions.filter(({ value }) => intervals.includes(value))
		: chartIntervalOptions;
	const [activeCandle, setActiveCandle] = useState<ChartCandle | null>(null);
	const [comparing, setComparing] = useState(false);
	usePriceComparison(comparison, comparing);
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
	} = usePriceHistory(source, interval);
	const { panes, shapes } = series;
	const themeColor = useThemeColor();
	const candleMarkers = createCandleMarkers(markers, shapes, themeColor);
	const lines = (placement: ChartPlacement) =>
		shapes.flatMap((shape) =>
			shape.kind === "line" && shape.placement === placement ? [shape] : [],
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

	const selectInterval = (value: CandleInterval) => {
		setActiveCandle(null);
		setInterval(value);
	};
	const readoutCandle = activeCandle ?? candles.at(-1);
	const legendSlotIndex = activeCandle
		? series.candles.findIndex((slot) => slot.time === activeCandle.time)
		: series.candles.length - 1;
	const legendItems = createIndicatorLegend(series, legendSlotIndex);
	const hasCandles = candles.length > 0;
	const candleHeight = compact
		? Math.max(
				candlePaneHeight,
				Math.round(screenHeight * compactCandlePaneShare),
			)
		: candlePaneHeight;
	const paneHeight = compact ? compactIndicatorPaneHeight : indicatorPaneHeight;
	const chartHeight = candleHeight + panes.length * paneHeight;
	const baseOptions = timeChartOptions[colorScheme][interval];
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
				<Group gap="xs" mx={compact ? paperPadding : 0} wrap="nowrap">
					<SegmentedControl<CandleInterval>
						data={intervalOptions}
						flex={1}
						fullWidth
						onChange={selectInterval}
						value={interval}
					/>
					{comparison ? (
						<Button
							onClick={() => setComparing(!comparing)}
							variant={comparing ? "filled" : "default"}
						>
							{comparison.label}
						</Button>
					) : null}
				</Group>
				<Box pos="relative" style={fillStyle}>
					{!hasCandles && !isLoading ? (
						<Text c="dimmed">No closed candles are available.</Text>
					) : null}
					<ChartCanvas
						key={`${symbol}:${interval}`}
						aria-label={`${symbol}: ${interval} candlestick history${live ? " with the current live candle" : " of the completed backtest"}. ${candles.length} candles loaded.${hasMore ? " Scroll left to load older candles." : " Earliest stored candle reached."}`}
						onVisibleLogicalRangeChange={onVisibleLogicalRangeChange}
						options={{
							...baseOptions,
							// The left scale belongs to the comparison line.
							leftPriceScale: {
								...baseOptions.leftPriceScale,
								visible: comparing,
							},
						}}
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
							markers={candleMarkers}
							onActiveCandleChange={setActiveCandle}
							priceLevels={priceLevels}
							priceResolution={series.priceResolution}
							visibleRange={visibleRange}
						/>
						{lines("overlay").map((shape) => (
							<LineShapeSeries key={shape.id} shape={shape} />
						))}
						{comparison && comparing ? (
							<ComparisonSeries comparison={comparison} interval={interval} />
						) : null}
						{lines("volume").map((shape) => (
							<LineShapeSeries key={shape.id} shape={shape} />
						))}
						{lines("pane").map((shape) => (
							<LineShapeSeries key={shape.id} shape={shape} />
						))}
						{/* Added after the lines whose panes and scales they share, so the
						lines keep the format of their scales. Markers on the candles are
						among the markers of the candles. */}
						{shapes.map((shape) => {
							switch (shape.kind) {
								case "area":
									return <AreaShapeSeries key={shape.id} shape={shape} />;
								case "markers":
									return <MarkersShapeSeries key={shape.id} shape={shape} />;
								case "segments":
									return <SegmentsShapeSeries key={shape.id} shape={shape} />;
							}
							return null;
						})}
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
						{readoutCandle ? <ChartReadout candle={readoutCandle} /> : null}
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
