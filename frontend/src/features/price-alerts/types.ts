export type PriceAlertEditor = {
	// The alert being edited; undefined while a new alert is entered.
	editingID: number | undefined;
	// What the number input emits: a number, or its text whenever a number
	// would change it, such as more than 13 digits or trailing zeros.
	target: number | string;
};

// The live price of the coin the alerts are for, which the target changes are
// measured from.
export interface CurrentPriceSource {
	subscribe(listener: () => void): () => void;
	getCurrentPrice(): number | undefined;
}
