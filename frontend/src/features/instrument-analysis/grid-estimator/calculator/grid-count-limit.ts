import type Decimal from "decimal.js";

import type { GridLimitsResponse } from "@/api/generated/models";
import { GridDecimal } from "@/features/instrument-analysis/grid-estimator/calculator/grid";
import type { GridType } from "@/features/instrument-analysis/grid-estimator/calculator/types";
import type { GridMarket } from "@/features/instrument-analysis/grid-estimator/types";

// The fewest grids a Binance grid bot accepts.
export const BINANCE_GRID_MIN_COUNT = 2;
// The default maker fees Binance grid bots check the grid step against before
// login: spot `baseTradingFeeRate` and the futures one.
const SPOT_GRID_FEE = "0.001";
const FUTURES_GRID_FEE = "0.0002";
// The most grids a Binance bot accepts: spot and USDT-M `maxGridCount`, and
// the COIN-M count Binance hardcodes.
const SPOT_GRID_MAX_COUNT = 500;
const USDM_GRID_MAX_COUNT = 1000;
const COINM_GRID_MAX_COUNT = 169;
// USDT-M grids of a tick size at or below the threshold keep their levels
// `priceDiffBuffer` ticks apart.
const USDM_SMALL_TICK_SIZE = new GridDecimal("0.00001");
const USDM_SMALL_TICK_MULTIPLIER = 10;

/** What a Binance grid bot's largest grid count depends on besides the range. */
export interface GridCountLimitParams {
	// The most grids of the market.
	cap: number;
	// The fee rate every grid step must cover.
	fee: Decimal;
	// How many ticks apart neighbouring levels must be at least.
	tickMultiplier: number;
	tickSize: Decimal;
}

// The natural logarithm in double precision, as Binance takes it.
const doubleLog = (value: Decimal) => Math.log(value.toNumber());

/**
 * Returns the largest grid count a Binance grid bot accepts for the range from
 * `lower` to `upper`: its levels stay at least `tickMultiplier` ticks apart,
 * every step covers the fee both ways, and the count stays within the
 * market's cap. Returns 0 for an invalid range or parameters.
 */
export function gridMaxCount(
	upper: Decimal,
	lower: Decimal,
	gridType: GridType,
	{ cap, fee, tickMultiplier, tickSize }: GridCountLimitParams,
): number {
	if (
		!lower.gt(0) ||
		!upper.gt(0) ||
		!tickSize.gt(0) ||
		!upper.gt(lower) ||
		!(cap > 0)
	)
		return 0;
	const interval = tickSize.times(tickMultiplier);
	if (gridType === "arithmetic") {
		const span = upper.minus(lower);
		const byTick = span.div(interval).floor().toNumber();
		const feeStep = fee
			.times(upper)
			.times(2)
			.toDecimalPlaces(tickSize.decimalPlaces(), GridDecimal.ROUND_UP);
		const byFee = span
			.div(feeStep.gt(0) ? feeStep : tickSize)
			.floor()
			.toNumber();
		return Math.min(byTick, byFee, cap);
	}
	const rangeLog = doubleLog(upper.div(lower));
	const one = new GridDecimal(1);
	const byTick = Math.floor(
		rangeLog / doubleLog(one.plus(interval.div(lower))),
	);
	const byFee = fee.gt(0)
		? Math.floor(rangeLog / doubleLog(one.plus(fee).div(one.minus(fee))))
		: cap;
	return Math.min(byTick, byFee, cap);
}

/**
 * Returns the grid count limit parameters of a market's Binance grid bot, or
 * null without the market's Binance limits.
 */
export function marketGridCountLimitParams(
	market: GridMarket,
	limits: GridLimitsResponse | null,
): GridCountLimitParams | null {
	if (market === "spot") {
		return limits
			? {
					cap: SPOT_GRID_MAX_COUNT,
					fee: new GridDecimal(SPOT_GRID_FEE),
					tickMultiplier: 1,
					tickSize: new GridDecimal(limits.spot.tick_size),
				}
			: null;
	}
	if (market === "usdm") {
		const usdm = limits?.usdm;
		if (!usdm) return null;
		const tickSize = new GridDecimal(usdm.tick_size);
		return {
			cap: USDM_GRID_MAX_COUNT,
			fee: new GridDecimal(FUTURES_GRID_FEE),
			tickMultiplier: tickSize.gt(USDM_SMALL_TICK_SIZE)
				? 1
				: USDM_SMALL_TICK_MULTIPLIER,
			tickSize,
		};
	}
	const coinm = limits?.coinm;
	return coinm
		? {
				cap: COINM_GRID_MAX_COUNT,
				fee: new GridDecimal(FUTURES_GRID_FEE),
				tickMultiplier: 1,
				tickSize: new GridDecimal(coinm.tick_size),
			}
		: null;
}
