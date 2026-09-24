import { type IPriceLine, LineStyle } from "lightweight-charts";
import { useLayoutEffect, useRef } from "react";
import { useSeriesLifecycle } from "@/components/lightweight-chart/context";
import type { ChartPriceLineOptions } from "@/components/lightweight-chart/types";

interface PriceLineProps {
	options: ChartPriceLineOptions;
}

export function PriceLine({ options }: PriceLineProps) {
	const parent = useSeriesLifecycle();
	const optionsRef = useRef(options);
	const priceLineRef = useRef<IPriceLine | null>(null);
	useLayoutEffect(() => {
		optionsRef.current = options;
	}, [options]);

	useLayoutEffect(() => {
		priceLineRef.current = parent.api().createPriceLine({
			...optionsRef.current,
			axisLabelVisible: true,
			lineStyle: LineStyle.Dashed,
			lineWidth: 1,
		});
		return () => {
			const priceLine = priceLineRef.current;
			priceLineRef.current = null;
			if (priceLine !== null && !parent.isRemoved) {
				parent.removePriceLine(priceLine);
			}
		};
	}, [parent]);

	useLayoutEffect(() => {
		priceLineRef.current?.applyOptions(options);
	}, [options]);

	return null;
}
