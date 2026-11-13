import { useEffect } from "react";

import type { PriceComparison } from "@/features/candle-chart/types";

// Streams the comparison while its line is on the chart.
export function usePriceComparison(
	comparison: PriceComparison | undefined,
	enabled: boolean,
): void {
	useEffect(() => {
		if (!comparison || !enabled) return;
		comparison.setEnabled(true);
		return () => comparison.setEnabled(false);
	}, [comparison, enabled]);
}
