import { useEffect, useSyncExternalStore } from "react";

import type {
	ChartInterval,
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/features/candle-chart/types";

// Reads the interval on screen from the source, which its owner starts.
export function usePriceHistory(
	source: PriceHistorySource,
	interval: ChartInterval,
): PriceHistorySnapshot {
	const snapshot = useSyncExternalStore(
		source.subscribe,
		() => source.getSnapshot(interval),
		() => source.getSnapshot(interval),
	);
	useEffect(() => {
		source.show(interval);
	}, [source, interval]);
	return snapshot;
}
