import {
	formatInputNumber,
	lowerPriceFromMarkup,
	lowerPriceLimitError,
	upperPriceFromMarkup,
	upperPriceLimitError,
	validPositiveNumber,
} from "@/features/instrument-analysis/grid-estimator/bounds";
import {
	DEFAULT_GRID_COUNT,
	DEFAULT_INVESTMENT,
	DEFAULT_MARKUP_PERCENT,
	DEFAULT_RANGE_MINIMUM_GRIDS,
	DEFAULT_STEP_HIGH_RANGE_RATIO,
	DEFAULT_STEP_LOW_RANGE_RATIO,
	DEFAULT_STEP_MAX_POSITION,
} from "@/features/instrument-analysis/grid-estimator/config";
import { isInverseMarket } from "@/features/instrument-analysis/grid-estimator/estimate";
import {
	gridCountForRange,
	gridCountForStep,
} from "@/features/instrument-analysis/grid-estimator/grid-range";
import type {
	GridBounds,
	GridEstimatorFormValues,
	GridMarket,
	GridRecommendation,
	SavedGridForm,
} from "@/features/instrument-analysis/grid-estimator/types";
import { GridDecimal, gridRangeRatio } from "@/utils/calculator/grid";
import type { GridType } from "@/utils/calculator/types";

/**
 * Returns the default minimum grid step between the hourly and the daily range:
 * the larger their ratio, the closer to the daily range. It stays at the hourly
 * range unless the daily range is valid and larger.
 */
export function defaultGridStepPercent(
	hourlyRange: number | undefined,
	dailyRange: number | undefined,
): number | undefined {
	if (
		!validPositiveNumber(hourlyRange) ||
		!validPositiveNumber(dailyRange) ||
		dailyRange <= hourlyRange
	) {
		return hourlyRange;
	}
	const share =
		(dailyRange / hourlyRange - DEFAULT_STEP_LOW_RANGE_RATIO) /
		(DEFAULT_STEP_HIGH_RANGE_RATIO - DEFAULT_STEP_LOW_RANGE_RATIO);
	const position = DEFAULT_STEP_MAX_POSITION * Math.min(Math.max(share, 0), 1);
	return hourlyRange + position * (dailyRange - hourlyRange);
}

/**
 * Keeps a saved form's values under a new step, with the grid count of that
 * step. A typed price outside the Binance limits never became the committed
 * one, so its markup gives the last accepted price.
 */
export function savedGridFormValues(
	{ bounds, values }: SavedGridForm,
	rangePercent: number,
): GridEstimatorFormValues {
	const lowerPrice = lowerPriceLimitError(bounds, values.lowerPrice, "")
		? (lowerPriceFromMarkup(bounds, values.lowerMarkup) ?? values.lowerPrice)
		: values.lowerPrice;
	const upperPrice = upperPriceLimitError(bounds, values.upperPrice, "")
		? (upperPriceFromMarkup(bounds, values.markup) ?? values.upperPrice)
		: values.upperPrice;
	const { gridCount } = gridCountForRange(
		{ ...values, lowerPrice, rangePercent, upperPrice },
		values.gridCount,
		bounds.anchor,
		values.lowerMarkup,
	);
	return { ...values, gridCount, lowerPrice, rangePercent, upperPrice };
}

// The markup of both prices: the default one, or wider when the range would
// not fit the minimum number of steps.
function defaultMarkupPercent(
	stepPercent: number | undefined,
	gridType: GridType,
): number {
	if (!validPositiveNumber(stepPercent)) return DEFAULT_MARKUP_PERCENT;
	const ratio = gridRangeRatio(
		new GridDecimal(stepPercent).div(100),
		DEFAULT_RANGE_MINIMUM_GRIDS,
		gridType,
	);
	if (!ratio) return DEFAULT_MARKUP_PERCENT;
	const markup = ratio
		.minus(1)
		.div(ratio.plus(1))
		.times(100)
		.toDecimalPlaces(2, GridDecimal.ROUND_CEIL)
		.toNumber();
	return Math.max(DEFAULT_MARKUP_PERCENT, markup);
}

/**
 * Starts both prices at the default markup within the range; the grid count is
 * the largest that keeps the step.
 */
export function gridRecommendation(
	bounds: GridBounds,
	stepPercent: number | undefined,
	gridType: GridType,
	investment: string,
): GridRecommendation {
	const markup = defaultMarkupPercent(stepPercent, gridType);
	const upperMarkup = Math.min(markup, bounds.upperMarkupMax);
	const upperPrice = upperPriceFromMarkup(bounds, upperMarkup) ?? "";
	const lowerMarkup = Math.min(markup, bounds.lowerMarkupMax);
	const lowerPrice = lowerPriceFromMarkup(bounds, lowerMarkup) ?? "";
	const gridCount =
		gridCountForStep(
			upperPrice,
			lowerPrice,
			stepPercent,
			gridType,
			bounds.anchor,
			lowerMarkup,
		) ?? DEFAULT_GRID_COUNT;
	return {
		input: {
			lowerPrice,
			upperPrice,
			gridCount,
			investment,
		},
		lowerMarkup,
		upperMarkup,
	};
}

/**
 * Returns the starting investment of a market: the default USDT amount, or for
 * COIN-M its value in the coin at the anchor price, or one coin without it.
 */
export function defaultInvestment(
	market: GridMarket,
	anchor: number | null,
): string {
	return investmentFromUsdt(market, anchor, DEFAULT_INVESTMENT) ?? "1";
}

/**
 * Converts a USDT amount into the market's margin asset at the anchor price,
 * or returns null when an inverse market has no price to convert at.
 */
export function investmentFromUsdt(
	market: GridMarket,
	anchor: number | null,
	usdt: string,
): string | null {
	if (!isInverseMarket(market)) return usdt;
	if (!validPositiveNumber(anchor)) return null;
	return formatInputNumber(new GridDecimal(usdt).div(anchor));
}
