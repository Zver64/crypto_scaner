import type Decimal from "decimal.js";

export type PositionDirection = "long" | "short";

export interface LinearAverageEntryPriceFill {
	quantity: number;
	price: number;
}

export interface UsdmAverageEntryPriceFill {
	price: Decimal.Value;
	quoteNotional: Decimal.Value;
}

export interface InverseAverageEntryPriceFill {
	contractCount: number;
	contractSize: number;
	direction?: PositionDirection;
	price: number;
}

export interface CoinmIsolatedLiquidationPriceOptions {
	direction: PositionDirection;
	contractCount: number;
	contractSize: number;
	entryPrice: number;
	isolatedWalletBalance: number;
	maintenanceMarginRatio: number;
}

export interface UsdmIsolatedLiquidationPriceInput {
	direction: PositionDirection;
	entryPrice: number;
	leverage: number;
	maintenanceMarginRatio: number;
	quantity: number;
}
