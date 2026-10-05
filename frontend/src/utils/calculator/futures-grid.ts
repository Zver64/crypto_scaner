import type Decimal from "decimal.js";
import {
	assertSupportedSpotGridValue,
	type GridType,
	parseSpotGridCount,
	parseSpotGridDecimal,
	SpotGridDecimal,
	type SpotGridInput,
} from "@/utils/calculator/spot-grid";
import type { PositionDirection } from "@/utils/calculator/types";

// A common futures maker fee; grid orders are limit orders.
export const FUTURES_GRID_FEE_RATE = new SpotGridDecimal("0.0002");
// A general maintenance margin rate rather than a per-symbol exchange bracket,
// conservative for most symbols' first bracket.
export const FUTURES_GRID_MAINTENANCE_MARGIN_RATE = new SpotGridDecimal("0.01");
export const FUTURES_GRID_MAX_LEVERAGE = 3;

// Linear contracts are margined and settled in the quote currency; inverse
// contracts have a fixed USD notional and are margined and settled in the coin.
export type FuturesContract = "linear" | "inverse";

export interface FuturesGridInput extends SpotGridInput {
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
	// Quote (USD) notional of each grid order.
	allocationPerOrder: Decimal;
	// Average entry price of the position at liquidation, or once every order
	// has filled when the position is never liquidated.
	averageEntryPrice: Decimal;
	filledOrderCount: number;
	gridCount: number;
	// Null when the position can never be liquidated.
	liquidationPrice: Decimal | null;
	lowerPrice: Decimal;
	stepPercentMaximum: Decimal;
	stepPercentMinimum: Decimal;
	tradeMaximum: FuturesGridTrade;
	tradeMinimum: FuturesGridTrade;
	upperPrice: Decimal;
}

function gridLevels(
	lowerPrice: Decimal,
	upperPrice: Decimal,
	gridCount: number,
	gridType: GridType,
): Decimal[] {
	const step = upperPrice.minus(lowerPrice).div(gridCount);
	const ratio =
		gridType === "geometric"
			? upperPrice.div(lowerPrice).pow(new SpotGridDecimal(1).div(gridCount))
			: null;
	const nextLevel = (level: Decimal) =>
		ratio ? level.times(ratio) : level.plus(step);
	const levels = [lowerPrice];
	for (let index = 1; index <= gridCount; index += 1) {
		// The upper level is exact rather than accumulated.
		const level =
			index === gridCount ? upperPrice : nextLevel(levels[index - 1]);
		if (!level.gt(levels[index - 1])) {
			throw new RangeError(
				"Price range is too narrow at the supported precision",
			);
		}
		levels.push(level);
	}
	return levels;
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
		FUTURES_GRID_FEE_RATE.times(new SpotGridDecimal(1).plus(closeLegShare)),
	);
	const openAmount = isLinear ? allocation : allocation.div(openPrice);
	const profit = openAmount.times(netReturn);
	assertSupportedSpotGridValue(profit, "Trade profit");
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
	const one = new SpotGridDecimal(1);
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
	assertSupportedSpotGridValue(price, "Liquidation price");
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
	const lowerPrice = parseSpotGridDecimal(input.lowerPrice, "Lower price");
	const upperPrice = parseSpotGridDecimal(input.upperPrice, "Upper price");
	const investment = parseSpotGridDecimal(input.investment, "Investment");
	const gridCount = parseSpotGridCount(input.gridCount);
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
	if (!upperPrice.gt(lowerPrice)) {
		throw new RangeError("Upper price must be greater than lower price");
	}
	const currentPrice = new SpotGridDecimal(input.currentPrice);
	const isLong = input.direction === "long";

	const levels = gridLevels(lowerPrice, upperPrice, gridCount, input.gridType);
	const isLinear = input.contract === "linear";
	const allocationPerOrder = investment
		.times(input.leverage)
		.times(isLinear ? 1 : currentPrice)
		.div(gridCount);
	assertSupportedSpotGridValue(
		allocationPerOrder,
		"Allocation per order",
		true,
	);

	// Long buy orders sit on every level but the upper one, short sell orders
	// on every level but the lower one, in the order the price reaches them.
	const orderPrices = isLong ? levels.slice(0, -1).reverse() : levels.slice(1);
	const filledAtMarket = (price: Decimal) =>
		isLong ? price.gte(currentPrice) : price.lte(currentPrice);
	const reached = (liquidationPrice: Decimal, price: Decimal) =>
		isLong ? liquidationPrice.gte(price) : liquidationPrice.lte(price);

	let quantity = new SpotGridDecimal(0);
	let notional = new SpotGridDecimal(0);
	let liquidationPrice: Decimal | null = null;
	let filledOrderCount = 0;
	for (const orderPrice of orderPrices) {
		const fillPrice = filledAtMarket(orderPrice) ? currentPrice : orderPrice;
		if (liquidationPrice && reached(liquidationPrice, fillPrice)) break;
		quantity = quantity.plus(allocationPerOrder.div(fillPrice));
		notional = notional.plus(allocationPerOrder);
		filledOrderCount += 1;
		liquidationPrice = liquidationPriceOf(
			input.contract,
			isLong,
			quantity,
			notional,
			investment,
		);
	}
	assertSupportedSpotGridValue(quantity, "Position quantity", true);
	const averageEntryPrice = notional.div(quantity);
	assertSupportedSpotGridValue(averageEntryPrice, "Average entry price", true);

	let tradeMinimum: FuturesGridTrade | undefined;
	let tradeMaximum: FuturesGridTrade | undefined;
	let stepPercentMinimum: Decimal | undefined;
	let stepPercentMaximum: Decimal | undefined;
	for (let index = 0; index < gridCount; index += 1) {
		const lower = levels[index];
		const upper = levels[index + 1];
		const trade = gridTrade(
			lower,
			upper,
			input.direction,
			input.contract,
			allocationPerOrder,
		);
		if (!tradeMinimum || trade.profit.lt(tradeMinimum.profit))
			tradeMinimum = trade;
		if (!tradeMaximum || trade.profit.gt(tradeMaximum.profit))
			tradeMaximum = trade;
		const stepPercent = upper.minus(lower).div(lower).times(100);
		if (!stepPercentMinimum || stepPercent.lt(stepPercentMinimum))
			stepPercentMinimum = stepPercent;
		if (!stepPercentMaximum || stepPercent.gt(stepPercentMaximum))
			stepPercentMaximum = stepPercent;
	}

	return {
		allocationPerOrder,
		averageEntryPrice,
		filledOrderCount,
		gridCount,
		liquidationPrice,
		lowerPrice,
		stepPercentMaximum: stepPercentMaximum as Decimal,
		stepPercentMinimum: stepPercentMinimum as Decimal,
		tradeMaximum: tradeMaximum as FuturesGridTrade,
		tradeMinimum: tradeMinimum as FuturesGridTrade,
		upperPrice,
	};
}
