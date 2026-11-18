import { Text } from "@mantine/core";
import { useSyncExternalStore } from "react";

import type { CurrentPriceSource } from "@/features/price-alerts/types";
import {
	formatTargetChange,
	targetChangePercent,
} from "@/features/price-alerts/utils";

interface TargetChangeProps {
	source: CurrentPriceSource;
	target: number | string;
}

// Subscribes on its own so live ticks re-render only the percentage. A fixed
// width keeps the input from resizing as the value changes.
export function TargetChange({ source, target }: TargetChangeProps) {
	const price = useSyncExternalStore(
		source.subscribe,
		source.getCurrentPrice,
		source.getCurrentPrice,
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
