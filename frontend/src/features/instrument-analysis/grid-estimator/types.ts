import type Decimal from "decimal.js";
import type { ArithmeticSpotGridEstimate } from "@/utils/calculator/arithmetic-spot-grid";
import type { FuturesGridEstimate } from "@/utils/calculator/futures-grid";
import type { GeometricSpotGridEstimate } from "@/utils/calculator/geometric-spot-grid";
import type { GridType, SpotGridInput } from "@/utils/calculator/spot-grid";
import type { PositionDirection } from "@/utils/calculator/types";

export type SpotGridEstimate =
	| ArithmeticSpotGridEstimate
	| GeometricSpotGridEstimate;

export interface GridCalculation<Estimate> {
	error: string | null;
	estimate: Estimate | null;
}

export type SpotGridCalculation = GridCalculation<SpotGridEstimate>;

export interface SpotGridProfitSplit {
	cleanProfit: string;
	// Net return of the trade relative to its order, e.g. "0.52% per trade".
	cleanReturnPercent: string;
	cleanSegmentPercent: number;
	feeCost: string;
	feeSegmentPercent: number;
	feeShareOfGross: string;
	grossProfit: string;
	isLoss: boolean;
	label: string;
}

export type PriceRounding = "ceil" | "floor" | undefined;

/** Binance spot grid limits of a symbol, from the grid-limits endpoint. */
export interface SpotGridLimits {
	askMultiplierUp: number;
	averagePrice: number;
	bidMultiplierDown: number;
	maxPrice: number;
	minPrice: number;
	tickSize: number;
}

/**
 * The price both markups are measured from and the price range a Binance grid
 * bot accepts. Without the Binance limits the anchor is the latest hourly
 * close and the prices are not limited.
 */
export interface SpotGridBounds {
	anchor: number | null;
	lowerMarkupMax: number;
	maxPrice: Decimal | null;
	minPrice: Decimal | null;
	tickSize: Decimal | null;
	upperMarkupMax: number;
}

export interface SpotGridRecommendation {
	input: SpotGridInput;
	hasAnchor: boolean;
	hasHourlyVolatility: boolean;
	lowerMarkup: number;
	upperMarkup: number;
}

export interface FuturesGridOptions {
	currentPrice: number | null;
	direction: PositionDirection;
	gridType: GridType;
	leverage: number;
}

export interface LiquidationRangeBarLayout {
	// Positions along the bar, in percent of its width.
	currentPosition: number | null;
	gridEndPosition: number;
	gridStartPosition: number;
	isInsideGrid: boolean;
	liquidationPosition: number | null;
	summary: string;
}

export interface GridEstimateValues {
	profitSplits: SpotGridProfitSplit[];
}

export type FuturesMarket = "usdm" | "coinm";

export type GridMarket = "spot" | FuturesMarket;

/** A spot, USDT-M, or COIN-M estimate, formatted for display. */
export interface GridMarketEstimate {
	error: string | null;
	futuresEstimate: FuturesGridEstimate | null;
	values: GridEstimateValues;
}

/** Markups from the anchor price, in percent. */
export interface GridMarkups {
	lower?: number;
	upper: number;
}
