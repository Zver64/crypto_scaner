import { formatCompactNumber } from "@/utils/number-format";

// Market cap inputs take millions of USD.
export const usdPerMillion = 1_000_000;

export function formatMarketCapUsd(value: number): string {
	return `$${formatCompactNumber(value)}`;
}
