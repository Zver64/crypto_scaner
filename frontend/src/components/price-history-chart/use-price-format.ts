import type { PriceFormatCustom } from "lightweight-charts";
import { useMemo } from "react";
import type { ChartCandleSlot } from "@/components/price-history-chart/types";
import {
	chartPriceResolution,
	formatPrice,
} from "@/components/price-history-chart/utils";

// Price format shared by the candles and every overlay on their price scale,
// so all axis labels in the candle pane look the same.
export function usePriceFormat(
	data: readonly ChartCandleSlot[],
): PriceFormatCustom {
	const { base, fractionDigits, minMove } = chartPriceResolution(data);
	// Depends on the resolution, not on data, so live ticks keep the same
	// object and the series are not reconfigured on every trade.
	return useMemo(
		() => ({
			base,
			// Axis ticks carry floating-point noise near zero; round it at the
			// chart resolution so it cannot widen the price scale.
			formatter: (value: number) => formatPrice(value, fractionDigits),
			minMove,
			type: "custom",
		}),
		[base, fractionDigits, minMove],
	);
}
