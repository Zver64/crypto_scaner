import { Text } from "@mantine/core";
import { useSyncExternalStore } from "react";
import type { CoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { formatNumber } from "@/utils/number-format";

// Subscribes on its own so live ticks re-render only the price.
export function CurrentPrice({ source }: { source: CoinChartData }) {
	const price = useSyncExternalStore(
		source.subscribe,
		source.getCurrentPrice,
		source.getCurrentPrice,
	);
	if (price === undefined) return null;
	return (
		<Text ff="monospace" fw={600}>
			{formatNumber(price)} USDT
		</Text>
	);
}
