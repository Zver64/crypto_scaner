import equal from "fast-deep-equal";
import type { CreatePriceLineOptions, IPriceLine } from "lightweight-charts";
import { useLayoutEffect, useRef } from "react";
import { useSeriesLifecycle } from "@/components/lightweight-chart/context";

interface PriceLineProps {
	options: CreatePriceLineOptions;
}

export function PriceLine({ options }: PriceLineProps) {
	const parent = useSeriesLifecycle();
	// The options the price line has: it is created with them and applies only
	// options that differ by value, so callers may build them on every render.
	const optionsRef = useRef(options);
	const priceLineRef = useRef<IPriceLine | null>(null);

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
		if (equal(optionsRef.current, options)) return;
		optionsRef.current = options;
		priceLineRef.current?.applyOptions(options);
	});

	return null;
}
