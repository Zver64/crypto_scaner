import type Decimal from "decimal.js";

import { GridDecimal } from "@/features/instrument-analysis/grid-estimator/calculator/grid";
import type { PositionDirection } from "@/features/instrument-analysis/grid-estimator/calculator/types";

// The margin share Binance futures grid bots keep free (`adjustCoef`).
export const FUTURES_GRID_ADJUST_COEF = new GridDecimal("0.8");
// Reserves of a Binance spot grid bot on top of its orders: a multiple of the
// minimum order value, of one order, or of the trading fee, and the share of
// the investment the bot spends on orders.
const SPOT_GRID_MIN_NOTIONAL_RESERVE = new GridDecimal("1.8");
const SPOT_GRID_ORDER_RESERVE = new GridDecimal("1.5");
const SPOT_GRID_FEE_RESERVE = new GridDecimal("0.003");
const SPOT_GRID_ORDER_SHARE = new GridDecimal("0.95");

/** Binance order limits of a spot symbol or a USDT-M contract. */
export interface GridOrderLimits {
	minNotional: Decimal;
	minQty: Decimal;
	stepSize: Decimal;
}

/** A futures grid's direction and leverage at the reference price. */
export interface FuturesGridPosition {
	direction: PositionDirection;
	leverage: number;
	// The mark price.
	price: Decimal;
}

/** Binance order limits of a COIN-M contract, in contracts. */
export interface CoinmGridOrderLimits {
	// The value of one contract, in USD.
	contractSize: Decimal;
	minQty: Decimal;
}

// Rounds `value` up to a multiple of `step`; a step of zero leaves it.
function ceilToStep(value: Decimal, step: Decimal): Decimal {
	return step.gt(0) ? value.toNearest(step, GridDecimal.ROUND_CEIL) : value;
}

function sum(values: readonly Decimal[]): Decimal {
	return values.reduce((total, value) => total.plus(value), new GridDecimal(0));
}

// Splits a futures grid's margin into the initial margin of every order and
// the loss of the orders filled at market on the far side of the price.
function futuresMinimum(
	initialMargin: Decimal,
	marketLoss: Decimal,
	leverage: number,
): Decimal {
	return initialMargin
		.div(FUTURES_GRID_ADJUST_COEF.times(leverage))
		.plus(marketLoss.div(FUTURES_GRID_ADJUST_COEF));
}

/**
 * Returns the smallest investment, in USDT, a Binance spot grid bot accepts
 * for `levels` (ascending, rounded to the tick size) at the average price
 * `price`, unrounded. Every order holds the minimum quantity; the levels below
 * the price are buy orders, and the bot buys the base of the levels above it
 * except one.
 */
export function spotGridMinimumInvestment(
	levels: readonly Decimal[],
	price: Decimal,
	{ minNotional, minQty, stepSize }: GridOrderLimits,
): Decimal {
	const quantity = ceilToStep(
		GridDecimal.max(minQty, minNotional.div(levels[0])),
		stepSize,
	);
	const buys = sum(levels.filter((level) => level.lt(price)));
	const sells = Math.max(
		0,
		levels.filter((level) => level.gte(price)).length - 1,
	);
	const orders = quantity.times(price.times(sells).plus(buys));
	const reserve = GridDecimal.max(
		minNotional.times(SPOT_GRID_MIN_NOTIONAL_RESERVE),
		quantity.times(price).times(SPOT_GRID_ORDER_RESERVE),
		orders.times(SPOT_GRID_FEE_RESERVE),
	);
	return orders.plus(reserve).div(SPOT_GRID_ORDER_SHARE);
}

/**
 * Returns the smallest investment, in USDT, a Binance USDT-M long or short
 * grid bot accepts for `levels` (ascending, rounded to the tick size),
 * unrounded. Every order holds the minimum quantity.
 */
export function usdmGridMinimumInvestment(
	levels: readonly Decimal[],
	{ direction, leverage, price }: FuturesGridPosition,
	{ minNotional, minQty, stepSize }: GridOrderLimits,
): Decimal {
	const quantity = GridDecimal.max(
		minQty,
		ceilToStep(minNotional.div(levels[0]), stepSize),
	);
	const isLong = direction === "long";
	const notional = sum(
		levels.map((level) => (isLong ? level : GridDecimal.max(level, price))),
	);
	const marketLoss = sum(
		levels.map((level) =>
			GridDecimal.max(0, isLong ? level.minus(price) : price.minus(level)),
		),
	);
	return futuresMinimum(
		quantity.times(notional),
		quantity.times(marketLoss),
		leverage,
	);
}

/**
 * Returns the smallest investment, in the coin, a Binance COIN-M long or short
 * grid bot accepts for `levels` (ascending, rounded to the tick size),
 * unrounded. Every order holds the minimum number of contracts.
 */
export function coinmGridMinimumInvestment(
	levels: readonly Decimal[],
	{ direction, leverage, price }: FuturesGridPosition,
	{ contractSize, minQty }: CoinmGridOrderLimits,
): Decimal {
	const value = minQty.times(contractSize);
	const isLong = direction === "long";
	const coinValue = (level: Decimal) => new GridDecimal(1).div(level);
	const notional = sum(
		levels.map((level) =>
			coinValue(isLong ? GridDecimal.min(level, price) : level),
		),
	);
	const marketLoss = sum(
		levels.map((level) =>
			GridDecimal.max(
				0,
				isLong
					? coinValue(price).minus(coinValue(level))
					: coinValue(level).minus(coinValue(price)),
			),
		),
	);
	return futuresMinimum(
		value.times(notional),
		value.times(marketLoss),
		leverage,
	);
}
