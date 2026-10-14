import { Group, Text } from "@mantine/core";
import type { ChartLegendItem } from "@/features/candle-chart/types";
import { useThemeColor } from "@/features/candle-chart/use-theme-color";

interface IndicatorLegendRowProps {
	items: readonly ChartLegendItem[];
}

export function IndicatorLegendRow({ items }: IndicatorLegendRowProps) {
	const themeColor = useThemeColor();
	if (items.length === 0) return null;
	return (
		<Group gap="xs" wrap="wrap">
			{items.map(({ color, key, title, value }) => (
				<Text
					c={themeColor(color)}
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
