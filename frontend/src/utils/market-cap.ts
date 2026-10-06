import { formatCompactNumber } from "@/utils/number-format";

export function formatMarketCapUsd(value: number): string {
	return `$${formatCompactNumber(value)}`;
}
