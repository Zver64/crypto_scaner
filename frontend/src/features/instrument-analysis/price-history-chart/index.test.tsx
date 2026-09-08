import { MantineProvider } from "@mantine/core";
import type { DeepPartial, TimeChartOptions } from "lightweight-charts";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, expect, it, vi } from "vitest";

const chartMocks = vi.hoisted(() => {
	const series = { setData: vi.fn() };
	const timeScale = { setVisibleRange: vi.fn() };
	const chart = {
		addSeries: vi.fn(() => series),
		remove: vi.fn(),
		subscribeCrosshairMove: vi.fn(),
		timeScale: vi.fn(() => timeScale),
		unsubscribeCrosshairMove: vi.fn(),
	};
	return {
		chart,
		createChart: vi.fn(
			(_container: HTMLElement, _options?: DeepPartial<TimeChartOptions>) =>
				chart,
		),
		series,
		timeScale,
	};
});

vi.mock("lightweight-charts", () => ({
	CandlestickSeries: Symbol("CandlestickSeries"),
	ColorType: { Solid: "solid" },
	createChart: chartMocks.createChart,
}));

import {
	InstrumentPriceHistoryChart,
	mountCandlestickChart,
} from "@/features/instrument-analysis/price-history-chart";
import type { ChartCandleSlot } from "@/features/instrument-analysis/price-history-chart/utils";

const window = { from: "2026-08-03T23:00:00Z", to: "2026-09-02T23:00:00Z" };
const candle = {
	close: 11,
	high: 12,
	low: 9,
	open: 10,
	open_time: window.to,
};

beforeEach(() => vi.clearAllMocks());

it("mounts with a seven-day default range and cleans up the chart", () => {
	const onCrosshair = vi.fn();
	const data: ChartCandleSlot[] = Array.from({ length: 721 }, (_, time) => ({
		time: time as never,
	}));
	data[720] = { ...candle, time: 720 as never };
	const cleanup = mountCandlestickChart({
		colors: {
			background: "black",
			down: "red",
			grid: "gray",
			text: "white",
			up: "green",
		},
		container: {} as HTMLElement,
		data,
		onCrosshair,
	});

	expect(chartMocks.createChart).toHaveBeenCalledOnce();
	expect(chartMocks.chart.addSeries).toHaveBeenCalledWith(
		expect.anything(),
		expect.objectContaining({
			priceFormat: expect.objectContaining({
				base: 1e7,
				minMove: 1e-7,
				type: "custom",
			}),
		}),
	);
	expect(chartMocks.series.setData).toHaveBeenCalledWith(data);
	expect(chartMocks.timeScale.setVisibleRange).toHaveBeenCalledWith({
		from: 552,
		to: 720,
	});
	expect(chartMocks.chart.subscribeCrosshairMove).toHaveBeenCalledOnce();
	const crosshairHandler =
		chartMocks.chart.subscribeCrosshairMove.mock.calls[0][0];
	crosshairHandler({
		seriesData: new Map([[chartMocks.series, { ...candle, time: 2 }]]),
	});
	expect(onCrosshair).toHaveBeenLastCalledWith({ ...candle, time: 2 });
	crosshairHandler({ seriesData: new Map() });
	expect(onCrosshair).toHaveBeenLastCalledWith(null);

	cleanup();
	expect(chartMocks.chart.unsubscribeCrosshairMove).toHaveBeenCalledOnce();
	expect(chartMocks.chart.remove).toHaveBeenCalledOnce();
});

it("allows all 721 slots to fit a 240px plot while initially showing the latest 169", () => {
	const data: ChartCandleSlot[] = Array.from({ length: 721 }, (_, time) => ({
		...candle,
		open: 1e-11,
		high: 1e-11,
		low: 1e-11,
		close: 1e-11,
		time: time as never,
	}));
	const cleanup = mountCandlestickChart({
		colors: {
			background: "black",
			down: "red",
			grid: "gray",
			text: "white",
			up: "green",
		},
		container: {} as HTMLElement,
		data,
		onCrosshair: vi.fn(),
	});
	const options = chartMocks.createChart.mock.calls[0][1];
	const minBarSpacing = options?.timeScale?.minBarSpacing ?? 0.5;
	// The time scale clamps requested spacing to minBarSpacing when zooming out.
	const plotWidth = 240;
	const fullHistorySpacing = Math.max(plotWidth / data.length, minBarSpacing);
	expect(plotWidth / fullHistorySpacing).toBeGreaterThanOrEqual(data.length);
	expect(chartMocks.series.setData).toHaveBeenCalledWith(data);
	expect(chartMocks.timeScale.setVisibleRange).toHaveBeenCalledExactlyOnceWith({
		from: 552,
		to: 720,
	});
	expect(chartMocks.chart.addSeries).toHaveBeenCalledWith(
		expect.anything(),
		expect.objectContaining({
			priceFormat: expect.objectContaining({ base: 1e18, minMove: 1e-18 }),
		}),
	);
	cleanup();
});

it("renders the latest OHLC fallback and an accessible chart summary", () => {
	const candles = Array(721).fill(null);
	candles[720] = candle;
	const html = renderToStaticMarkup(
		<MantineProvider>
			<InstrumentPriceHistoryChart
				candles={candles}
				symbol="BTCUSDT"
				window={window}
			/>
		</MantineProvider>,
	);

	expect(html).toContain("Sep 2, 23:00 UTC");
	expect(html).toContain("O 10  H 12  L 9  C 11");
	expect(html).toContain(
		"Up to 30 days of hourly candlestick prices. 1 of 721 hourly slots contain candles",
	);
});

it("renders a clear empty state without mounting a chart", () => {
	const html = renderToStaticMarkup(
		<MantineProvider>
			<InstrumentPriceHistoryChart
				candles={Array(721).fill(null)}
				symbol="BTCUSDT"
				window={window}
			/>
		</MantineProvider>,
	);

	expect(html).toContain("—");
	expect(html).toContain("No hourly candle history");
});
