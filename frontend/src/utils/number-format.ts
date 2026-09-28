type NumericValue = number | string;

const compactNumberFormatter = new Intl.NumberFormat("en", {
	maximumFractionDigits: 1,
	notation: "compact",
});

// Creating an Intl.NumberFormat is far more expensive than formatting with one,
// and chart axes call formatNumber for every label on every redraw. Keep one
// formatter per fraction digit limit instead of rebuilding it on each call.
const adaptiveNumberFormatters = new Map<number, Intl.NumberFormat>();

// Matches the backend NUMERIC(38,18) price scale, so real values keep their
// significant digits.
const defaultMaximumFractionDigits = 18;

// Intl formats numeric strings exactly, so Decimal output keeps its precision.
function toNumeric(value: NumericValue): number | Intl.StringNumericLiteral {
	return value as number | Intl.StringNumericLiteral;
}

// Three significant digits, optionally capped at a number of fraction digits so
// values below that resolution (such as floating-point noise) format as zero.
export function formatNumber(
	value: NumericValue,
	maximumFractionDigits = defaultMaximumFractionDigits,
): string {
	let formatter = adaptiveNumberFormatters.get(maximumFractionDigits);
	if (!formatter) {
		formatter = new Intl.NumberFormat("en", {
			maximumFractionDigits,
			maximumSignificantDigits: 3,
			roundingPriority: "lessPrecision",
			signDisplay: "negative",
		});
		adaptiveNumberFormatters.set(maximumFractionDigits, formatter);
	}
	return formatter.format(toNumeric(value));
}

export function formatCompactNumber(value: NumericValue): string {
	return compactNumberFormatter.format(toNumeric(value));
}
