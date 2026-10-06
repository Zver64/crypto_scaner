import { Stack } from "@mantine/core";
import { IndicatorLegendRow } from "@/features/candle-chart/indicator-legend-row";
import type { ChartLegendItem } from "@/features/candle-chart/types";

interface IndicatorLegendProps {
	items: readonly ChartLegendItem[];
}

// Indicator titles and values below the OHLC readout: overlays drawn on the
// candles on one line, indicators in panes below the chart on the next.
export function IndicatorLegend({ items }: IndicatorLegendProps) {
	if (items.length === 0) return null;
	const overlays = items.filter(({ placement }) => placement === "overlay");
	const panes = items.filter(({ placement }) => placement === "pane");
	return (
		<Stack gap={2} mb="xs">
			<IndicatorLegendRow items={overlays} />
			<IndicatorLegendRow items={panes} />
		</Stack>
	);
}
