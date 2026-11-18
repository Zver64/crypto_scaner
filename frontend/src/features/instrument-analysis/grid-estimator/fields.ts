import type {
	GridMarket,
	GridMarketFormValues,
} from "@/features/instrument-analysis/grid-estimator/types";

/** Returns the calculator form field of a market's value. */
export function marketField<Key extends keyof GridMarketFormValues>(
	market: GridMarket,
	key: Key,
) {
	return `markets.${market}.${key}` as const;
}
