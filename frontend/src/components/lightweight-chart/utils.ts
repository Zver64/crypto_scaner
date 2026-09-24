import type { ChartCandleSlot } from "@/components/lightweight-chart/types";

export function chartPriceResolution(data: readonly ChartCandleSlot[]): {
	base: number;
	minMove: number;
} {
	const prices = data.flatMap((item) =>
		"open" in item ? [item.open, item.high, item.low, item.close] : [],
	);
	const smallest = Math.min(...prices.filter((price) => price > 0));
	// Eight significant digits (at least cents), with a valid decimal tick base.
	const exponent = Number.isFinite(smallest)
		? Math.min(308, Math.max(2, 7 - Math.floor(Math.log10(smallest))))
		: 2;
	return { base: 10 ** exponent, minMove: 10 ** -exponent };
}
