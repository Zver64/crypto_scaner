import { Text, useMantineTheme } from "@mantine/core";

import { chartFontSize } from "@/features/candle-chart/config";
import type { PriceCandle } from "@/features/candle-chart/types";
import { formatCandleRange, formatOhlc } from "@/features/candle-chart/utils";

interface ChartReadoutProps {
	candle: Pick<PriceCandle, "open" | "high" | "low" | "close">;
}

export function ChartReadout({ candle }: ChartReadoutProps) {
	const { fontWeights } = useMantineTheme();
	return (
		<Text
			aria-live="polite"
			ff="monospace"
			// The size of the chart's own text.
			fz={{ base: chartFontSize.compact, sm: chartFontSize.regular }}
			size="xs"
		>
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
