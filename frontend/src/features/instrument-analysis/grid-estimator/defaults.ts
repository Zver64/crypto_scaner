import type { GridLimitsResponse } from "@/api/generated/models";
import {
	formatInputNumber,
	gridBounds,
	lowerPriceFromMarkup,
	lowerPriceLimitError,
	upperPriceFromMarkup,
	upperPriceLimitError,
	validPositiveNumber,
} from "@/features/instrument-analysis/grid-estimator/bounds";
import {
	GridDecimal,
	gridRangeRatio,
} from "@/features/instrument-analysis/grid-estimator/calculator/grid";
import type { GridType } from "@/features/instrument-analysis/grid-estimator/calculator/types";
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
	gridCountCap,
	gridCountForRange,
	gridCountForStep,
} from "@/features/instrument-analysis/grid-estimator/grid-range";
import type {
	GridBounds,
	GridCalculatorBuild,
	GridCalculatorFormValues,
	GridEstimatorFormValues,
	GridMarket,
	GridMarketFormValues,
	GridRecommendation,
} from "@/features/instrument-analysis/grid-estimator/types";

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
 * Keeps an edited market's values under a new step, with the grid count of
 * that step up to the market's Binance maximum. A typed price outside the
 * Binance limits never became the committed one, so its markup gives the last
 * accepted price.
 */
export function keptGridFormValues<Values extends GridEstimatorFormValues>(
	market: GridMarket,
	bounds: GridBounds,
	limits: GridLimitsResponse | null,
	values: Values,
	rangePercent: number,
): Values {
	const lowerPrice = lowerPriceLimitError(bounds, values.lowerPrice, "")
		? (lowerPriceFromMarkup(bounds, values.lowerMarkup) ?? values.lowerPrice)
		: values.lowerPrice;
	const upperPrice = upperPriceLimitError(bounds, values.upperPrice, "")
		? (upperPriceFromMarkup(bounds, values.markup) ?? values.upperPrice)
		: values.upperPrice;
	const range = { ...values, lowerPrice, rangePercent, upperPrice };
	const { gridCount } = gridCountForRange(
		range,
		values.gridCount,
		gridCountCap(market, range, limits),
		bounds.anchor,
		values.lowerMarkup,
	);
	return { ...values, gridCount, lowerPrice, rangePercent, upperPrice };
}

// A market built anew: both prices at the default markup, the grid count of
// the default step, and the market's default investment.
function newGridMarketValues(
	market: GridMarket,
	close: number | null,
	gridLimits: GridLimitsResponse | null,
	stepPercent: number | undefined,
): GridMarketFormValues {
	const bounds = gridBounds(market, close, gridLimits?.spot);
	const { input, lowerMarkup, upperMarkup } = gridRecommendation(
		market,
		bounds,
		gridLimits,
		stepPercent,
		"geometric",
		defaultInvestment(market, bounds.anchor),
	);
	return {
		...input,
		close,
		committed: input,
		direction: "short",
		edited: false,
		gridType: "geometric",
		leverage: 1,
		lowerMarkup,
		markup: upperMarkup,
		rangePercent: stepPercent ?? 0,
	};
}

/**
 * Builds the calculator for its data and volatility ranges. A market the user
 * has edited (`edited`, or changed since the last build) keeps its values and
 * bounds except the step and the grid count, which follow the new ranges, and
 * its typed values become the committed ones; any other market is built anew
 * from the latest close.
 */
export function gridCalculatorValues(
	build: GridCalculatorBuild,
	previous: GridCalculatorFormValues | null,
	changedMarkets: ReadonlySet<GridMarket>,
): GridCalculatorFormValues {
	const stepPercent = defaultGridStepPercent(
		build.hourlyVolatilityPercent,
		build.dailyVolatilityPercent,
	);
	const rangePercent = stepPercent ?? 0;
	const marketValues = (market: GridMarket): GridMarketFormValues => {
		const values = previous?.markets[market];
		if (!values || !(values.edited || changedMarkets.has(market))) {
			return newGridMarketValues(
				market,
				build.close,
				build.gridLimits,
				stepPercent,
			);
		}
		const kept = keptGridFormValues(
			market,
			gridBounds(market, values.close, build.gridLimits?.spot),
			build.gridLimits,
			values,
			rangePercent,
		);
		const { gridCount, investment, lowerPrice, upperPrice } = kept;
		return {
			...kept,
			committed: { gridCount, investment, lowerPrice, upperPrice },
			edited: true,
		};
	};
	return {
		builtFor: build.builtFor,
		defaultRangePercent: rangePercent,
		market: previous?.market ?? "spot",
		markets: {
			coinm: marketValues("coinm"),
			spot: marketValues("spot"),
			usdm: marketValues("usdm"),
		},
	};
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
 * the largest that keeps the step, or the default one without a step, up to
 * the market's Binance maximum.
 */
export function gridRecommendation(
	market: GridMarket,
	bounds: GridBounds,
	limits: GridLimitsResponse | null,
	stepPercent: number | undefined,
	gridType: GridType,
	investment: string,
): GridRecommendation {
	const markup = defaultMarkupPercent(stepPercent, gridType);
	const upperMarkup = Math.min(markup, bounds.upperMarkupMax);
	const upperPrice = upperPriceFromMarkup(bounds, upperMarkup) ?? "";
	const lowerMarkup = Math.min(markup, bounds.lowerMarkupMax);
	const lowerPrice = lowerPriceFromMarkup(bounds, lowerMarkup) ?? "";
	const maxCount = gridCountCap(
		market,
		{ gridType, lowerPrice, upperPrice },
		limits,
	);
	const gridCount =
		gridCountForStep(
			upperPrice,
			lowerPrice,
			stepPercent,
			gridType,
			maxCount,
			bounds.anchor,
			lowerMarkup,
		) ?? String(Math.min(Number(DEFAULT_GRID_COUNT), maxCount));
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
