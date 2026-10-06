import { useComputedColorScheme } from "@mantine/core";
import { PriceLine } from "@/components/lightweight-chart";
import { minMaxPriceLineOptions } from "@/features/candle-chart/config";

interface MinMaxPriceLinesProps {
	max: number;
	min: number;
}

export function MinMaxPriceLines({ max, min }: MinMaxPriceLinesProps) {
	const options = minMaxPriceLineOptions[useComputedColorScheme("dark")];
	if (min === max) {
		return (
			<PriceLine options={{ ...options, price: min, title: "Min / Max" }} />
		);
	}
	return (
		<>
			<PriceLine options={{ ...options, price: min, title: "Min" }} />
			<PriceLine options={{ ...options, price: max, title: "Max" }} />
		</>
	);
}
