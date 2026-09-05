export function sevenDayChangePercent(
	prices: readonly (number | null)[],
): number | null {
	const available = prices.filter((price): price is number => price !== null);
	const first = available[0];
	const last = available.at(-1);

	if (first === undefined || last === undefined || first === 0) {
		return null;
	}

	return ((last - first) / first) * 100;
}
