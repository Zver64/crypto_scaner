import type { CreatePriceLineOptions, IPriceLine } from "lightweight-charts";
import { useLayoutEffect, useRef } from "react";
import { useSeriesLifecycle } from "@/components/lightweight-chart/context";

interface PriceLineProps {
	options: CreatePriceLineOptions;
}

export function PriceLine({ options }: PriceLineProps) {
	const parent = useSeriesLifecycle();
	const optionsRef = useRef(options);
	const priceLineRef = useRef<IPriceLine | null>(null);
	useLayoutEffect(() => {
		optionsRef.current = options;
	}, [options]);

	useLayoutEffect(() => {
		priceLineRef.current = parent.api().createPriceLine(optionsRef.current);
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
