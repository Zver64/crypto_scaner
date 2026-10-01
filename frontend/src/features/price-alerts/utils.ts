import Decimal from "decimal.js";
import type { PriceAlert } from "@/api/generated/models";
import { normalizePriceTarget } from "@/features/price-alerts/target";
import { formatRangePercent } from "@/utils/range-percent";

// Percent distance from the current price to the target; undefined until the
// price is known and the target is one the alert form would accept.
export function targetChangePercent(
	target: string,
	price: number | undefined,
): number | undefined {
	if (price === undefined || !(price > 0) || !Number.isFinite(price)) {
		return undefined;
	}
	const normalized = normalizePriceTarget(target);
	if ("error" in normalized) return undefined;
	const percent = ((Number(normalized.value) - price) / price) * 100;
	return Number.isFinite(percent) ? percent : undefined;
}

export function formatTargetChange(percent: number): string {
	return `${percent > 0 ? "+" : ""}${formatRangePercent(percent)}`;
}

// Targets keep up to 18 decimals, so compare them as decimals.
export function sortAlertsDescending(
	alerts: readonly PriceAlert[],
): PriceAlert[] {
	return [...alerts].sort((a, b) => new Decimal(b.target).comparedTo(a.target));
}
