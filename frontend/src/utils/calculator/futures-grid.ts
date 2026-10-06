import type Decimal from "decimal.js";
import {
	assertSupportedGridValue,
	extremes,
	GridDecimal,
	gridLevels,
	parseGridInput,
} from "@/utils/calculator/grid";
import type {
	GridInput,
	GridType,
	PositionDirection,
} from "@/utils/calculator/types";

// A common futures maker fee; grid orders are limit orders.
export const FUTURES_GRID_FEE_RATE = new GridDecimal("0.0002");
// A general maintenance margin rate rather than a per-symbol exchange bracket,
// conservative for most symbols' first bracket.
export const FUTURES_GRID_MAINTENANCE_MARGIN_RATE = new GridDecimal("0.01");
export const FUTURES_GRID_MAX_LEVERAGE = 3;

// Linear contracts are margined and settled in the quote currency; inverse
// contracts have a fixed USD notional and are margined and settled in the coin.
export type FuturesContract = "linear" | "inverse";

export interface FuturesGridInput extends GridInput {
	contract: FuturesContract;
	// The price the bot starts at; orders on its far side fill at market.
	currentPrice: number;
	direction: PositionDirection;
	gridType: GridType;
	leverage: number;
}

// Amounts are in the margin currency: the quote for linear contracts, the
// coin for inverse ones.
export interface FuturesGridTrade {
	grossProfit: Decimal;
	profit: Decimal;
	profitPercent: Decimal;
}

export interface FuturesGridEstimate {
	// Null when the position can never be liquidated.
	liquidationPrice: Decimal | null;
	lowerPrice: Decimal;
	stepPercentMaximum: Decimal;
	stepPercentMinimum: Decimal;
	tradeMaximum: FuturesGridTrade;
	tradeMinimum: FuturesGridTrade;
	upperPrice: Decimal;
}

// Net result of one grid trade relative to its opening order in the margin
// currency: a long buys at `lower` and sells at `upper`, a short sells at
// `upper` and buys back at `lower`. Fees are charged on the notional of both
// legs. A linear order opens `allocation` quote; an inverse order opens
// `allocation / openPrice` coin and gains `allocation × (1/lower − 1/upper)`.
function gridTrade(
	lower: Decimal,
	upper: Decimal,
	direction: PositionDirection,
	contract: FuturesContract,
	allocation: Decimal,
): FuturesGridTrade {
	const openPrice = direction === "long" ? lower : upper;
	const closePrice = direction === "long" ? upper : lower;
	const isLinear = contract === "linear";
	const grossReturn = upper.minus(lower).div(isLinear ? openPrice : closePrice);
	// Fees per margin unit of the opening order: the closing leg's notional in
	// the margin currency, relative to the opening leg's.
	const closeLegShare = isLinear
		? closePrice.div(openPrice)
		: openPrice.div(closePrice);
	const netReturn = grossReturn.minus(
		FUTURES_GRID_FEE_RATE.times(new GridDecimal(1).plus(closeLegShare)),
	);
	const openAmount = isLinear ? allocation : allocation.div(openPrice);
	const profit = openAmount.times(netReturn);
	assertSupportedGridValue(profit, "Trade profit");
	return {
		grossProfit: openAmount.times(grossReturn),
		profit,
		profitPercent: netReturn.times(100),
	};
}

// Returns the price at which the isolated margin balance equals the
// maintenance margin, or null when the position can never be liquidated.
// `quantity` is the position size in coins, `notional` its entry value in the
// quote, and `margin` is in the quote for linear and in the coin for inverse
// contracts.
function liquidationPriceOf(
	contract: FuturesContract,
	isLong: boolean,
	quantity: Decimal,
	notional: Decimal,
	margin: Decimal,
): Decimal | null {
	const one = new GridDecimal(1);
	const mmr = FUTURES_GRID_MAINTENANCE_MARGIN_RATE;
	let price: Decimal;
	if (contract === "linear") {
		// WB ± Q·(LP − EP) = MMR·Q·LP.
		price = isLong
			? notional.minus(margin).div(quantity.times(one.minus(mmr)))
			: notional.plus(margin).div(quantity.times(one.plus(mmr)));
	} else {
		// In coins, with N the notional: WB ± (Q − N/LP) = MMR·N/LP.
		const denominator = isLong ? margin.plus(quantity) : quantity.minus(margin);
		if (!denominator.gt(0)) return null;
		price = notional
			.times(isLong ? one.plus(mmr) : one.minus(mmr))
			.div(denominator);
	}
	assertSupportedGridValue(price, "Liquidation price");
	return price.gt(0) ? price : null;
}

/**
 * Estimates a futures grid with equal quote (USD) notional per order and the
 * whole investment as isolated margin: in the quote for linear contracts, in
 * the coin for inverse ones, converted at the current price. The liquidation price assumes
 * the price moves straight against the grid without a completed trade: orders
 * on the far side of the current price fill at market when the bot starts,
 * the rest fill one by one as their level is reached, and the position is
 * liquidated as soon as the price reaches its liquidation price, possibly
 * before every order has filled. Fees are left out of the liquidation.
 */
export function calculateFuturesGrid(
	input: FuturesGridInput,
): FuturesGridEstimate {
	const { gridCount, investment, lowerPrice, upperPrice } =
		parseGridInput(input);
	if (!Number.isFinite(input.currentPrice) || input.currentPrice <= 0) {
		throw new RangeError("Current price must be a positive number");
	}
	if (
		!Number.isFinite(input.leverage) ||
		input.leverage < 1 ||
		input.leverage > FUTURES_GRID_MAX_LEVERAGE
	) {
		throw new RangeError(
			`Leverage must be from 1 to ${FUTURES_GRID_MAX_LEVERAGE}`,
		);
	}
	const currentPrice = new GridDecimal(input.currentPrice);
	const isLong = input.direction === "long";

	const levels = gridLevels(lowerPrice, upperPrice, gridCount, input.gridType);
	const isLinear = input.contract === "linear";
	// Quote (USD) notional of each grid order.
	const allocationPerOrder = investment
		.times(input.leverage)
		.times(isLinear ? 1 : currentPrice)
		.div(gridCount);
	assertSupportedGridValue(allocationPerOrder, "Allocation per order", true);

	// Long buy orders sit on every level but the upper one, short sell orders
	// on every level but the lower one, in the order the price reaches them.
	const orderPrices = isLong ? levels.slice(0, -1).reverse() : levels.slice(1);
	const filledAtMarket = (price: Decimal) =>
		isLong ? price.gte(currentPrice) : price.lte(currentPrice);
	const reached = (liquidationPrice: Decimal, price: Decimal) =>
		isLong ? liquidationPrice.gte(price) : liquidationPrice.lte(price);

	let quantity = new GridDecimal(0);
	let notional = new GridDecimal(0);
	let liquidationPrice: Decimal | null = null;
	for (const orderPrice of orderPrices) {
		const fillPrice = filledAtMarket(orderPrice) ? currentPrice : orderPrice;
		if (liquidationPrice && reached(liquidationPrice, fillPrice)) break;
		quantity = quantity.plus(allocationPerOrder.div(fillPrice));
		notional = notional.plus(allocationPerOrder);
		liquidationPrice = liquidationPriceOf(
			input.contract,
			isLong,
			quantity,
			notional,
			investment,
		);
	}
	assertSupportedGridValue(quantity, "Position quantity", true);

	const steps = levels.slice(0, -1).map((lower, index) => {
		const upper = levels[index + 1];
		return {
			percent: upper.minus(lower).div(lower).times(100),
			trade: gridTrade(
				lower,
				upper,
				input.direction,
				input.contract,
				allocationPerOrder,
			),
		};
	});
	const trades = extremes(steps, (step) => step.trade.profit);
	const stepPercents = extremes(steps, (step) => step.percent);

	return {
		liquidationPrice,
		lowerPrice,
		stepPercentMaximum: stepPercents.maximum.percent,
		stepPercentMinimum: stepPercents.minimum.percent,
		tradeMaximum: trades.maximum.trade,
		tradeMinimum: trades.minimum.trade,
		upperPrice,
	};
}
