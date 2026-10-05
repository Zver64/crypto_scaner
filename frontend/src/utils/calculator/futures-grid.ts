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

// Binance USDⓈ-M regular-user maker fee; grid orders are limit orders.
export const FUTURES_GRID_FEE_RATE = new SpotGridDecimal("0.0002");
// A general maintenance margin rate rather than a per-symbol Binance bracket,
// conservative for most symbols' first bracket.
export const FUTURES_GRID_MAINTENANCE_MARGIN_RATE = new SpotGridDecimal("0.01");
export const FUTURES_GRID_MAX_LEVERAGE = 3;

export interface FuturesGridInput extends SpotGridInput {
	// The price the bot starts at; orders on its far side fill at market.
	currentPrice: number;
	direction: PositionDirection;
	gridType: GridType;
	leverage: number;
}

export interface FuturesGridTrade {
	grossProfitPercent: Decimal;
	profit: Decimal;
	profitPercent: Decimal;
}

export interface FuturesGridEstimate {
	// Quote notional of each grid order.
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

// Net result of one grid trade per quote unit of its opening order: a long
// buys at `lower` and sells at `upper`, a short sells at `upper` and buys back
// at `lower`. Fees are charged on the notional of both legs.
function gridTrade(
	lower: Decimal,
	upper: Decimal,
	direction: PositionDirection,
	allocation: Decimal,
): FuturesGridTrade {
	const openPrice = direction === "long" ? lower : upper;
	const closePrice = direction === "long" ? upper : lower;
	const grossReturn = upper.minus(lower).div(openPrice);
	const netReturn = grossReturn.minus(
		FUTURES_GRID_FEE_RATE.times(
			new SpotGridDecimal(1).plus(closePrice.div(openPrice)),
		),
	);
	const profit = allocation.times(netReturn);
	assertSupportedSpotGridValue(profit, "Trade profit");
	return {
		grossProfitPercent: grossReturn.times(100),
		profit,
		profitPercent: netReturn.times(100),
	};
}

/**
 * Estimates a Binance USDⓈ-M futures grid with equal quote notional per order
 * and the whole investment as isolated margin. The liquidation price assumes
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
	const allocationPerOrder = investment.times(input.leverage).div(gridCount);
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
		// Isolated margin balance at the liquidation price equals the
		// maintenance margin: WB ± Q·(LP − EP) = MMR·Q·LP.
		liquidationPrice = isLong
			? notional
					.minus(investment)
					.div(
						quantity.times(
							new SpotGridDecimal(1).minus(
								FUTURES_GRID_MAINTENANCE_MARGIN_RATE,
							),
						),
					)
			: notional
					.plus(investment)
					.div(
						quantity.times(
							new SpotGridDecimal(1).plus(FUTURES_GRID_MAINTENANCE_MARGIN_RATE),
						),
					);
	}
	assertSupportedSpotGridValue(quantity, "Position quantity", true);
	const averageEntryPrice = notional.div(quantity);
	assertSupportedSpotGridValue(averageEntryPrice, "Average entry price", true);
	if (liquidationPrice) {
		assertSupportedSpotGridValue(liquidationPrice, "Liquidation price");
		if (!liquidationPrice.gt(0)) liquidationPrice = null;
	}

	let tradeMinimum: FuturesGridTrade | undefined;
	let tradeMaximum: FuturesGridTrade | undefined;
	let stepPercentMinimum: Decimal | undefined;
	let stepPercentMaximum: Decimal | undefined;
	for (let index = 0; index < gridCount; index += 1) {
		const lower = levels[index];
		const upper = levels[index + 1];
		const trade = gridTrade(lower, upper, input.direction, allocationPerOrder);
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
