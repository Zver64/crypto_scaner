import { Text, useMantineTheme } from "@mantine/core";
import type {
	ChartReadoutOptions,
	PriceCandle,
} from "@/components/price-history-chart/types";
import { formatOhlc } from "@/components/price-history-chart/utils";

interface ChartReadoutProps {
	candle: Pick<PriceCandle, "open" | "high" | "low" | "close">;
	extra?: ChartReadoutOptions;
}

export function ChartReadout({ candle, extra }: ChartReadoutProps) {
	const { fontWeights } = useMantineTheme();
	return (
		<Text aria-live="polite" ff="monospace" mb="xs" size="xs">
			{formatOhlc(candle)}
			{extra ? (
				<>
					{" · "}
					<Text
						c="blue.4"
						component="span"
						fw={fontWeights.bold}
						inherit
						style={{ whiteSpace: "nowrap" }}
					>
						{extra.label} {extra.format(candle)}
					</Text>
				</>
			) : null}
		</Text>
	);
}
