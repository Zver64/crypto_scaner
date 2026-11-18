import type { GridLimitsResponse } from "@/api/generated/models";
import { GRID_MARKET_TABS } from "@/features/instrument-analysis/grid-estimator/config";
import { gridCountRange } from "@/features/instrument-analysis/grid-estimator/estimate";
import { marketField } from "@/features/instrument-analysis/grid-estimator/fields";
import type {
	GridCalculatorFormValues,
	GridMarket,
	GridMarketFormValues,
} from "@/features/instrument-analysis/grid-estimator/types";

/** Returns why a market's typed grid count is outside the Binance range. */
function gridCountLimitError(
	market: GridMarket,
	{ committed, gridCount, gridType }: GridMarketFormValues,
	limits: GridLimitsResponse | null,
): string | undefined {
	const range = gridCountRange(market, committed, gridType, limits);
	if (!range) return undefined;
	if (range.tooNarrow) return "Price range is too narrow for a grid";
	if (!/^\d+$/.test(gridCount)) return undefined;
	const count = Number(gridCount);
	if (count < range.min) return `Minimum is ${range.min} grids`;
	if (count > range.max) return `Maximum is ${range.max} grids`;
	return undefined;
}

/**
 * The calculator form's check of every market's typed grid count against the
 * Binance range of its committed grid, so the error follows both.
 */
export function gridCountLimitErrors(
	values: GridCalculatorFormValues,
	limits: GridLimitsResponse | null,
) {
	return {
		fields: Object.fromEntries(
			GRID_MARKET_TABS.map(({ value: market }) => [
				marketField(market, "gridCount"),
				gridCountLimitError(market, values.markets[market], limits),
			]).filter(([, error]) => error !== undefined),
		),
	};
}
