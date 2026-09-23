import Decimal from "decimal.js";

export type PriceTargetResult =
	| { value: string; error?: never }
	| { value?: never; error: string };

export function normalizePriceTarget(input: string): PriceTargetResult {
	const raw = input.trim();
	if (!raw) return { error: "Enter a target price." };
	try {
		const decimal = new Decimal(raw);
		if (!decimal.isFinite() || !decimal.gt(0)) {
			return { error: "Target must be greater than zero." };
		}
		if (decimal.decimalPlaces() > 18) {
			return { error: "Target can have at most 18 decimal places." };
		}
		const value = decimal.toFixed(decimal.decimalPlaces());
		const [integer, fraction = ""] = value.split(".");
		if (integer.length > 20 || integer.length + fraction.length > 38) {
			return { error: "Target price is too large." };
		}
		return { value };
	} catch {
		return { error: "Enter a valid decimal price." };
	}
}
