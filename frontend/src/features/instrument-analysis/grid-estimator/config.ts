import type {
	FuturesMarket,
	GridMarket,
	GridMarkups,
} from "@/features/instrument-analysis/grid-estimator/types";
import type { FuturesContract } from "@/utils/calculator/futures-grid";
import { SpotGridDecimal } from "@/utils/calculator/spot-grid";

// Starting markups of each market. Without a lower markup, the lower price
// starts as low as the range allows.
export const DEFAULT_MARKUPS: Record<GridMarket, GridMarkups> = {
	spot: { upper: 5 },
	usdm: { lower: 10, upper: 10 },
	coinm: { lower: 10, upper: 10 },
};
export const UPPER_MARKUP_MAX_PERCENT = 50;
export const LOWER_MARKUP_MAX_PERCENT = 50;
// Slider caps when the Binance limits allow more: the lower price stays above
// zero, and the upper price goes no further than triple the average price.
export const LOWER_MARKUP_LIMIT_PERCENT = 99;
export const UPPER_MARKUP_LIMIT_PERCENT = 200;
export const DEFAULT_GRID_COUNT = "40";
export const DEFAULT_INVESTMENT = "1000";
// Investment buttons, in USDT; COIN-M grids invest their worth in the coin.
export const INVESTMENT_PRESETS = ["100", "500", "1000", "2000"] as const;

export const tickRounding = {
	ceil: SpotGridDecimal.ROUND_CEIL,
	floor: SpotGridDecimal.ROUND_FLOOR,
	halfExpand: SpotGridDecimal.ROUND_HALF_UP,
};

// Binance grid bots keep their orders inside this share of the deviation the
// exchange's percent price filter allows.
export const BINANCE_GRID_FILTER_SHARE = 0.85;
// How far from the average price the range may go when a symbol has no
// percent price filter.
export const BINANCE_GRID_FALLBACK_DEVIATION = 0.5;

// The contract each futures market trades; inverse contracts are margined in
// the coin.
export const FUTURES_CONTRACTS: Record<FuturesMarket, FuturesContract> = {
	usdm: "linear",
	coinm: "inverse",
};

// Share of the shown span added on each side, so markers at the ends stay
// visible.
export const RANGE_BAR_PADDING = 0.05;
