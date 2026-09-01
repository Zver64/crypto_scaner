export type PositionDirection = "long" | "short";

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
