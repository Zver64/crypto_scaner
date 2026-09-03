const rangePercentFormatter = new Intl.NumberFormat("en", {
	maximumSignificantDigits: 3,
});

export function formatRangePercent(value: number): string {
	return `${rangePercentFormatter.format(value)}%`;
}
