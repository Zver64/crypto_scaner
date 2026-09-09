import { formatNumber } from "@/utils/number-format";

export function formatRangePercent(value: number): string {
	return `${formatNumber(value)}%`;
}
