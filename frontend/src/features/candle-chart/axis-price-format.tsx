import { useLayoutEffect } from "react";
import { useSeriesLifecycle } from "@/components/lightweight-chart";
import type { ValueResolution } from "@/features/candle-chart/types";
import { axisPriceFormat } from "@/features/candle-chart/utils";

interface AxisPriceFormatProps {
	resolution: ValueResolution;
}

// The axis price format of the series it is inside. Its formatter is created
// and applied only when the resolution's values change, so live ticks, which
// bring a new resolution object, do not reconfigure the series.
export function AxisPriceFormat({
	resolution: { base, fractionDigits, minMove },
}: AxisPriceFormatProps) {
	const series = useSeriesLifecycle();
	useLayoutEffect(() => {
		series.api().applyOptions({
			priceFormat: axisPriceFormat({ base, fractionDigits, minMove }),
		});
	}, [series, base, fractionDigits, minMove]);
	return null;
}
