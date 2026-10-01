import { Group, Stack, Text } from "@mantine/core";
import type { ChartLegendItem } from "@/components/price-history-chart/types";

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

function IndicatorLegendRow({ items }: IndicatorLegendProps) {
	if (items.length === 0) return null;
	return (
		<Group gap="xs" wrap="wrap">
			{items.map(({ color, key, title, value }) => (
				<Text
					c={color}
					ff="monospace"
					key={key}
					size="xs"
					style={{ whiteSpace: "nowrap" }}
				>
					{title} {value ?? "—"}
				</Text>
			))}
		</Group>
	);
}
