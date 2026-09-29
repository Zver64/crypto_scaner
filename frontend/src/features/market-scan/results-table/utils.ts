import type {
	TableRow,
	UnresolvedInstrumentCode,
} from "@/api/generated/models";

// Blends the positive and negative theme colors: 0 is fully green, 100 red.
export function oscillatorColor(value: number): string {
	const red = Math.min(100, Math.max(0, value));
	return `color-mix(in oklch, var(--mantine-color-red-6) ${red}%, var(--mantine-color-green-6))`;
}

export function filterMarketScanRows(
	rows: readonly TableRow[],
	filter: string,
): TableRow[] {
	const normalizedFilter = filter.trim().toLocaleLowerCase("en-US");
	return rows.filter((row) =>
		row.symbol.toLocaleLowerCase("en-US").includes(normalizedFilter),
	);
}

const marketCapUnavailableReasons: Record<UnresolvedInstrumentCode, string> = {
	mapping_conflict:
		"Multiple market capitalization matches were found for this instrument.",
	mapping_not_found:
		"No market capitalization match was found for this instrument.",
	mapping_provider_unavailable:
		"Market capitalization mapping data is temporarily unavailable.",
	market_cap_missing:
		"Market capitalization data is unavailable for this instrument.",
};

export function marketCapUnavailableReason(
	code: UnresolvedInstrumentCode,
): string {
	return marketCapUnavailableReasons[code];
}
