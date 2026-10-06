export type PositionDirection = "long" | "short";

export type GridType = "arithmetic" | "geometric";

/** Grid calculator input as typed: decimal strings and a whole grid count. */
export interface GridInput {
	gridCount: string;
	investment: string;
	lowerPrice: string;
	upperPrice: string;
}
