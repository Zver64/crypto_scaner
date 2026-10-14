export type PriceAlertEditor = {
	// The alert being edited; undefined while a new alert is entered.
	editingID: number | undefined;
	// What the number input emits: a number, or its text whenever a number
	// would change it, such as more than 13 digits or trailing zeros.
	target: number | string;
};
