type NumericValue = number | string;

interface NumericNumberFormatter {
	formatToParts(value: NumericValue): Intl.NumberFormatPart[];
}

// TypeScript exposes numeric-string formatting only with newer Intl library
// declarations. Browsers support it and avoid an imprecise Number conversion.
const adaptiveNumberFormatter = new Intl.NumberFormat("en", {
	maximumSignificantDigits: 3,
}) as unknown as NumericNumberFormatter;

const compactNumberFormatter = new Intl.NumberFormat("en", {
	maximumFractionDigits: 1,
	notation: "compact",
}) as unknown as NumericNumberFormatter;

function formatNumericValue(
	formatter: NumericNumberFormatter,
	value: NumericValue,
): string {
	const source = String(value);
	const parts = formatter.formatToParts(value);
	const sourceIsNonzero = /[1-9]/.test(source);
	const resultIsNonzero = parts
		.filter((part) => part.type === "integer" || part.type === "fraction")
		.some((part) => /[1-9]/.test(part.value));

	// Some engines coerce numeric strings to Number. Preserve the source instead
	// of displaying infinity or rounding a nonzero value down to zero.
	if (
		parts.some((part) => part.type === "infinity") ||
		(sourceIsNonzero && !resultIsNonzero)
	) {
		return source;
	}

	return parts.map((part) => part.value).join("");
}

export function formatNumber(value: NumericValue): string {
	return formatNumericValue(adaptiveNumberFormatter, value);
}

export function formatCompactNumber(value: NumericValue): string {
	return formatNumericValue(compactNumberFormatter, value);
}
