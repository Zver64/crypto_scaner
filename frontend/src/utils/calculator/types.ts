import type Decimal from "decimal.js";

export type PositionDirection = "long" | "short";

export interface LinearAverageEntryPriceFill {
	quantity: Decimal.Value;
	price: Decimal.Value;
}

export interface UsdmAverageEntryPriceFill {
	price: Decimal.Value;
	quoteNotional: Decimal.Value;
}

export interface InverseAverageEntryPriceFill {
	contractCount: number;
	contractSize: Decimal.Value;
	direction?: PositionDirection;
	price: Decimal.Value;
}

export interface CoinmIsolatedLiquidationPriceOptions {
	direction: PositionDirection;
	contractCount: number;
	contractSize: Decimal.Value;
	entryPrice: Decimal.Value;
	isolatedWalletBalance: Decimal.Value;
	maintenanceMarginRatio: Decimal.Value;
}

export interface UsdmIsolatedLiquidationPriceInput {
	direction: PositionDirection;
	entryPrice: Decimal.Value;
	leverage: Decimal.Value;
	maintenanceMarginRatio: Decimal.Value;
	quantity: Decimal.Value;
}
