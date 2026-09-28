import { Group, Text } from "@mantine/core";
import type { ChartLegendItem } from "@/components/price-history-chart/types";

interface IndicatorLegendProps {
	items: readonly ChartLegendItem[];
}

// Indicator titles and values, on their own line below the OHLC readout.
export function IndicatorLegend({ items }: IndicatorLegendProps) {
	if (items.length === 0) return null;
	return (
		<Group gap="xs" mb="xs" wrap="wrap">
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
