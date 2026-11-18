import type Decimal from "decimal.js";

import type { GridLimitsResponse } from "@/api/generated/models";
import { parseGridDecimal } from "@/features/instrument-analysis/grid-estimator/calculator/grid";
import { GRID_MARKET_TABS } from "@/features/instrument-analysis/grid-estimator/config";
import {
	gridMinimumInvestment,
	marginAsset,
} from "@/features/instrument-analysis/grid-estimator/estimate";
import { marketField } from "@/features/instrument-analysis/grid-estimator/fields";
import type {
	GridCalculatorFormValues,
	GridMarket,
	GridMarketFormValues,
} from "@/features/instrument-analysis/grid-estimator/types";
import { formatNumber } from "@/utils/number-format";

/**
 * Returns the smallest investment Binance accepts for a market's committed
 * grid and its text with the margin currency, or null without one. The text
 * keeps every decimal of the amount, as a typed price limit does.
 */
export function marketMinimumInvestment(
	market: GridMarket,
	{ committed, direction, gridType, leverage }: GridMarketFormValues,
	limits: GridLimitsResponse | null,
	baseAsset: string,
): { amount: Decimal; text: string } | null {
	const amount = gridMinimumInvestment(
		market,
		committed,
		{ direction, gridType, leverage },
		limits,
	);
	if (!amount) return null;
	const decimals = amount.decimalPlaces();
	const text = formatNumber(
		amount.toFixed(),
		decimals,
		"ceil",
		false,
		decimals,
	);
	return { amount, text: `${text} ${marginAsset(market, baseAsset)}` };
}

/** Returns why a market's typed investment is below the Binance minimum. */
function investmentMinimumError(
	market: GridMarket,
	values: GridMarketFormValues,
	limits: GridLimitsResponse | null,
	baseAsset: string,
): string | undefined {
	const minimum = marketMinimumInvestment(market, values, limits, baseAsset);
	if (!minimum) return undefined;
	try {
		return parseGridDecimal(values.investment, "Investment").lt(minimum.amount)
			? `Minimum is ${minimum.text}`
			: undefined;
	} catch {
		return undefined;
	}
}

/**
 * The calculator form's check of every market's typed investment against the
 * minimum of its committed grid, so the error follows both.
 */
export function investmentMinimumErrors(
	values: GridCalculatorFormValues,
	limits: GridLimitsResponse | null,
	baseAsset: string,
) {
	return {
		fields: Object.fromEntries(
			GRID_MARKET_TABS.map(({ value: market }) => [
				marketField(market, "investment"),
				investmentMinimumError(
					market,
					values.markets[market],
					limits,
					baseAsset,
				),
			]).filter(([, error]) => error !== undefined),
		),
	};
}
