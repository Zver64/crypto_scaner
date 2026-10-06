import type { ChartReadoutOptions } from "@/features/candle-chart";
import { formatRangePercent } from "@/utils/range-percent";

export const rangeReadout: ChartReadoutOptions = {
	label: "R",
	format: (candle) =>
		formatRangePercent(((candle.high - candle.low) / candle.open) * 100),
};
