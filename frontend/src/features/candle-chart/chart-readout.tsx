import { Text, useMantineTheme } from "@mantine/core";

import type { PriceCandle } from "@/features/candle-chart/types";
import { formatCandleRange, formatOhlc } from "@/features/candle-chart/utils";

interface ChartReadoutProps {
	candle: Pick<PriceCandle, "open" | "high" | "low" | "close">;
}

export function ChartReadout({ candle }: ChartReadoutProps) {
	const { fontWeights } = useMantineTheme();
	return (
		<Text aria-live="polite" ff="monospace" size="xs">
			{formatOhlc(candle)}
			{" · "}
			<Text
				c="blue.4"
				component="span"
				fw={fontWeights.bold}
				inherit
				style={{ whiteSpace: "nowrap" }}
			>
				R {formatCandleRange(candle)}
			</Text>
		</Text>
	);
}
