import { Group, Text } from "@mantine/core";
import type { ChartLegendItem } from "@/features/candle-chart/types";

interface IndicatorLegendRowProps {
	items: readonly ChartLegendItem[];
}

export function IndicatorLegendRow({ items }: IndicatorLegendRowProps) {
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
