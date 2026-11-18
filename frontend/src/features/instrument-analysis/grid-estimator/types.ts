import type Decimal from "decimal.js";
import type { ComponentProps } from "react";

import type { SegmentedValueGroup } from "@/components/segmented-value-group";
import type { ArithmeticSpotGridEstimate } from "@/features/instrument-analysis/grid-estimator/calculator/arithmetic-spot-grid";
import type { FuturesGridEstimate } from "@/features/instrument-analysis/grid-estimator/calculator/futures-grid";
import type { GeometricSpotGridEstimate } from "@/features/instrument-analysis/grid-estimator/calculator/geometric-spot-grid";
import type {
	GridInput,
	GridType,
	PositionDirection,
} from "@/features/instrument-analysis/grid-estimator/calculator/types";

export type SpotGridEstimate =
	| ArithmeticSpotGridEstimate
	| GeometricSpotGridEstimate;

export interface GridCalculation<Estimate> {
	error: string | null;
	estimate: Estimate | null;
}

export type SpotGridCalculation = GridCalculation<SpotGridEstimate>;

export interface GridProfitSplit {
	cleanProfit: string;
	// Net return of the trade relative to its order, e.g. "0.52% per trade".
	cleanReturnPercent: string;
	cleanSegmentPercent: number;
	feeCost: string;
	feeSegmentPercent: number;
	feeShareOfGross: string;
	isLoss: boolean;
	// Absent when every trade of the grid shows the same result.
	label?: string;
}

/** The amounts of one grid trade, in the margin currency. */
export interface GridTradeAmounts {
	grossProfit: Decimal;
	profit: Decimal;
	profitPercent: Decimal;
}

export type ProfitSplitRow = ComponentProps<
	typeof SegmentedValueGroup
>["rows"][number];

/** Theme colors of the profit bar segments. */
export interface ProfitSplitColors {
	fee: string;
	profit: string;
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
 * The price both markups are measured from and, for spot grids, the price
 * range a Binance spot grid bot accepts. Without the Binance limits the anchor
 * is the latest hourly close and the prices are not limited.
 */
export interface GridBounds {
	anchor: number | null;
	lowerMarkupMax: number;
	maxPrice: Decimal | null;
	minPrice: Decimal | null;
	tickSize: Decimal | null;
	upperMarkupMax: number;
}

export interface GridRecommendation {
	input: GridInput;
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
	profitSplits: GridProfitSplit[];
}

export type FuturesMarket = "usdm" | "coinm";

export type GridMarket = "spot" | FuturesMarket;

/** A spot, USDT-M, or COIN-M estimate, formatted for display. */
export interface GridMarketEstimate {
	error: string | null;
	futuresEstimate: FuturesGridEstimate | null;
	values: GridEstimateValues;
}

export type GridEstimatorFormValues = GridInput & {
	// Futures only.
	direction: PositionDirection;
	gridType: GridType;
	// Futures only.
	leverage: number;
	lowerMarkup: number;
	markup: number;
	rangePercent: number;
};

// One market's calculator: the values its inputs hold and the input its
// estimate and chart levels are calculated from.
export type GridMarketFormValues = GridEstimatorFormValues & {
	// The latest hourly close when the market was built; its bounds start from
	// it without the Binance limits.
	close: number | null;
	// The typed prices, grid count, and investment as last accepted.
	committed: GridInput;
	// Whether the user changed the market's values on this page; new ranges
	// recalculate only the step and the grid count of an edited market.
	edited: boolean;
};

// The calculator of a coin page: every market and the selected one.
export interface GridCalculatorFormValues {
	// The readiness and volatility ranges the values were built for.
	builtFor: string;
	// The default minimum step of those ranges, marked on the step slider.
	defaultRangePercent: number;
	market: GridMarket;
	markets: Record<GridMarket, GridMarketFormValues>;
}

// What the calculator is built from when its data is ready or new volatility
// ranges arrive.
export interface GridCalculatorBuild {
	builtFor: string;
	// The latest hourly close.
	close: number | null;
	dailyVolatilityPercent: number | undefined;
	gridLimits: SpotGridLimits | null;
	hourlyVolatilityPercent: number | undefined;
}

export type GridInputField = keyof GridInput;

export type GridRangeValues = Pick<
	GridEstimatorFormValues,
	"gridType" | "lowerPrice" | "rangePercent" | "upperPrice"
>;

/** A range's grid count, or the kept count and why none was derived. */
export interface GridRangeCount {
	error: string | null;
	gridCount: string;
}
