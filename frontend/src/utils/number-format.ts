type NumericValue = number | string;

const compactNumberFormatter = new Intl.NumberFormat("en", {
	maximumFractionDigits: 1,
	notation: "compact",
});

type RoundingMode = NonNullable<Intl.NumberFormatOptions["roundingMode"]>;

// Creating an Intl.NumberFormat is far more expensive than formatting with one,
// and chart axes call formatNumber for every label on every redraw. Keep one
// formatter per option set instead of rebuilding it on each call.
const adaptiveNumberFormatters = new Map<RoundingMode, Intl.NumberFormat>();
const fractionLimitFormatters = new Map<string, Intl.NumberFormat>();

// Matches the backend NUMERIC(38,18) price scale, so real values keep their
// significant digits.
const defaultMaximumFractionDigits = 18;

// Intl formats numeric strings exactly, so Decimal output keeps its precision.
function toNumeric(value: NumericValue): number | Intl.StringNumericLiteral {
	return value as number | Intl.StringNumericLiteral;
}

function cachedFormatter<K>(
	cache: Map<K, Intl.NumberFormat>,
	key: K,
	options: Intl.NumberFormatOptions,
): Intl.NumberFormat {
	let formatter = cache.get(key);
	if (!formatter) {
		formatter = new Intl.NumberFormat("en", options);
		cache.set(key, formatter);
	}
	return formatter;
}

// Keeps the whole integer part and lets Intl pick whichever of "whole number"
// or "four significant digits" shows more, so 5785.4 formats as 5,785 and
// 0.012345 as 0.01235. An optional fraction digit limit then drops values below
// that resolution (such as floating-point noise) to zero. Pass useGrouping
// false for input values, which must not contain group separators. A minimum
// fraction digit count opts into fixed fractional formatting without the adaptive
// significant-digit rounding, and pads trailing zeros.
export function formatNumber(
	value: NumericValue,
	maximumFractionDigits = defaultMaximumFractionDigits,
	roundingMode: RoundingMode = "halfExpand",
	useGrouping = true,
	minimumFractionDigits = 0,
): string {
	const formattedValue =
		minimumFractionDigits > 0
			? toNumeric(value)
			: (cachedFormatter(adaptiveNumberFormatters, roundingMode, {
					maximumFractionDigits: 0,
					maximumSignificantDigits: 4,
					roundingMode,
					roundingPriority: "morePrecision",
					useGrouping: false,
				}).format(toNumeric(value)) as Intl.StringNumericLiteral);
	return cachedFormatter(
		fractionLimitFormatters,
		`${maximumFractionDigits}:${roundingMode}:${useGrouping}:${minimumFractionDigits}`,
		{
			maximumFractionDigits,
			minimumFractionDigits,
			roundingMode,
			signDisplay: "negative",
			useGrouping,
		},
	).format(formattedValue);
}

export function formatCompactNumber(value: NumericValue): string {
	return compactNumberFormatter.format(toNumeric(value));
}
