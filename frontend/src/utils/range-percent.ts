import { formatNumber } from "@/utils/number-format";

// A percentage with formatNumber precision, optionally limited to fewer
// fraction digits.
export function formatRangePercent(
	value: number,
	maximumFractionDigits?: number,
): string {
	return `${formatNumber(value, maximumFractionDigits)}%`;
}

// The sign of a percentage as formatRangePercent shows it, so a value rounded
// to 0% has none.
export function displayedPercentSign(
	value: number,
	maximumFractionDigits?: number,
): -1 | 0 | 1 {
	if (formatNumber(value, maximumFractionDigits) === "0") return 0;
	return value > 0 ? 1 : -1;
}
