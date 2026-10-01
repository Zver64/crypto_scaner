import { Text } from "@mantine/core";
import { useSyncExternalStore } from "react";
import type { CoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import {
	formatTargetChange,
	targetChangePercent,
} from "@/features/price-alerts/utils";

interface TargetChangeProps {
	source?: CoinChartData;
	target: string;
}

const subscribeNothing = () => () => {};
const noPrice = () => undefined;

// Subscribes on its own so live ticks re-render only the percentage. A fixed
// width keeps the input from resizing as the value changes.
export function TargetChange({ source, target }: TargetChangeProps) {
	const getPrice = source?.getCurrentPrice ?? noPrice;
	const price = useSyncExternalStore(
		source?.subscribe ?? subscribeNothing,
		getPrice,
		getPrice,
	);
	const percent = targetChangePercent(target, price);
	return (
		<Text
			c={
				percent === undefined || percent === 0
					? "dimmed"
					: percent > 0
						? "teal"
						: "red"
			}
			ff="monospace"
			fw={600}
			flex="none"
			size="sm"
			ta="right"
			truncate="end"
			w="10ch"
		>
			{percent === undefined ? "-%" : formatTargetChange(percent)}
		</Text>
	);
}
