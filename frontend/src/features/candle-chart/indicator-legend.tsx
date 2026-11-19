import { Flex } from "@mantine/core";

import { IndicatorLegendGroup } from "@/features/candle-chart/indicator-legend-group";
import type { ChartLegendGroup } from "@/features/candle-chart/types";

interface IndicatorLegendProps {
	groups: readonly ChartLegendGroup[];
}

// Indicator titles and values below the OHLC readout over the chart, a little
// apart by indicator.
export function IndicatorLegend({ groups }: IndicatorLegendProps) {
	if (groups.length === 0) return null;
	return (
		<Flex columnGap="sm" rowGap={4} wrap="wrap">
			{groups.map(({ indicator, items }) => (
				<IndicatorLegendGroup items={items} key={indicator} />
			))}
		</Flex>
	);
}
