import { PriceLine } from "@/components/lightweight-chart";
import { priceLevelOptions } from "@/features/candle-chart/config";

interface PriceLevelLinesProps {
	currentPrice: number;
	// Ascending and distinct; the first and last are the bounds.
	levels: readonly number[];
}

export function PriceLevelLines({
	currentPrice,
	levels,
}: PriceLevelLinesProps) {
	const last = levels.length - 1;
	return levels.map((price, index) => (
		<PriceLine
			key={price}
			options={{
				...(index === 0
					? priceLevelOptions.lower
					: index === last
						? priceLevelOptions.upper
						: price < currentPrice
							? priceLevelOptions.stepBelow
							: priceLevelOptions.stepAbove),
				price,
			}}
		/>
	));
}
