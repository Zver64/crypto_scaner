import { useEffect, useSyncExternalStore } from "react";
import type {
	ChartInterval,
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/components/price-history-chart/types";

// Streams the interval on screen from the source while the chart is enabled.
export function usePriceHistory(
	source: PriceHistorySource,
	interval: ChartInterval,
	enabled: boolean,
): PriceHistorySnapshot {
	const snapshot = useSyncExternalStore(
		source.subscribe,
		() => source.getSnapshot(interval),
		() => source.getSnapshot(interval),
	);
	useEffect(() => {
		source.show(interval);
	}, [source, interval]);
	useEffect(() => {
		if (!enabled) return;
		source.start();
		return () => source.stop();
	}, [enabled, source]);
	return snapshot;
}
