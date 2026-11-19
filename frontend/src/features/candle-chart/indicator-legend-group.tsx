import { Flex, Text } from "@mantine/core";

import type { ChartLegendItem } from "@/features/candle-chart/types";
import { useThemeColor } from "@/features/candle-chart/use-theme-color";

interface IndicatorLegendGroupProps {
	items: readonly ChartLegendItem[];
}

// The legend entries of one indicator, close together.
export function IndicatorLegendGroup({ items }: IndicatorLegendGroupProps) {
	const themeColor = useThemeColor();
	return (
		<Flex columnGap={8} rowGap={0} wrap="wrap">
			{items.map(({ color, key, title, value }) => (
				<Text
					c={themeColor(color)}
					ff="monospace"
					// Smaller on phones, where the chart has little room.
					fz={{ base: 10, sm: "xs" }}
					key={key}
					size="xs"
					style={{ whiteSpace: "nowrap" }}
				>
					{title} {value ?? "—"}
				</Text>
			))}
		</Flex>
	);
}
