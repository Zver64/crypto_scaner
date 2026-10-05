import type Decimal from "decimal.js";

export type PositionDirection = "long" | "short";

export interface LinearAverageEntryPriceFill {
	quantity: Decimal.Value;
	price: Decimal.Value;
}
