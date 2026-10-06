import type { MantineVars } from "@mantine/vanilla-extract";
import type {
	TableRow,
	UnresolvedInstrumentCode,
} from "@/api/generated/models";
import { marketCapUnavailableReasons } from "@/features/market-scan/results-table/constants";

// Blends the positive and negative theme colors: 0 is fully green, 100 red.
export function oscillatorColor(
	value: number,
	colors: Pick<MantineVars["colors"], "green" | "red">,
): string {
	const red = Math.min(100, Math.max(0, value));
	return `color-mix(in oklch, ${colors.red[6]} ${red}%, ${colors.green[6]})`;
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

export function marketCapUnavailableReason(
	code: UnresolvedInstrumentCode,
): string {
	return marketCapUnavailableReasons[code];
}
