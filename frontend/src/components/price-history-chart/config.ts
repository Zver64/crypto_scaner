import { LineStyle } from "lightweight-charts";
import type { PriceHistoryChartConfig } from "@/components/price-history-chart/types";

export const priceHistoryChartConfig = {
	height: 300,
	// Candle pane and indicator pane heights, in that order.
	paneStretchFactors: [3, 1],
	viewport: {
		// Request older candles when fewer than this many bars remain on the left.
		loadOlderThreshold: 10,
		minVisibleBars: 24,
	},
	chart: {
		timeScale: {
			barSpacing: 7.5,
			minBarSpacing: 2,
			secondsVisible: false,
		},
	},
	candles: {
		priceScale: {
			// Library defaults leave 20% empty above and 10% below the candles.
			scaleMargins: { bottom: 0.05, top: 0.05 },
		},
		series: { borderVisible: false },
	},
	volume: {
		opacity: 0.4,
		priceScale: {
			// Volume bars use the bottom 20% of the candle pane, like TradingView.
			scaleMargins: { bottom: 0, top: 0.8 },
		},
		series: {
			lastValueVisible: false,
			priceFormat: { type: "volume" },
			priceLineVisible: false,
			// An empty id puts the series on its own hidden overlay price scale.
			priceScaleId: "",
		},
	},
	indicator: {
		priceScale: {
			autoScale: true,
			scaleMargins: { bottom: 0, top: 0 },
		},
		series: {
			lastValueVisible: true,
			lineWidth: 2,
			priceLineVisible: false,
		},
	},
	priceLine: {
		axisLabelVisible: true,
		lineStyle: LineStyle.Dashed,
		lineWidth: 1,
	},
} satisfies PriceHistoryChartConfig;
