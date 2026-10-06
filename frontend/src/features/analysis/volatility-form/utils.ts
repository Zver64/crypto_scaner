import type { NumberInputProps } from "@mantine/core";
import type { NumberInputField } from "@/components/number-input-fieldset";
import { analysisCriteriaConstraints } from "@/features/analysis/criteria";
import type {
	VolatilityFieldsetProps,
	VolatilityFormField,
} from "@/features/analysis/volatility-form/types";
import { marketScanCriteriaConstraints } from "@/features/market-scan/criteria";

export function buildVolatilityInputs({
	minimumRangePercent,
	minimumRangePresets,
	percentile,
	percentilePresets,
	period,
	periodPresets,
}: Omit<VolatilityFieldsetProps, "title">): NumberInputField[] {
	return [
		input(period, periodPresets, {
			allowDecimal: false,
			label: "Period",
			max: analysisCriteriaConstraints.period.maximum,
			min: analysisCriteriaConstraints.period.minimum,
		}),
		input(percentile, percentilePresets, {
			allowDecimal: false,
			label: "Percentile",
			max: analysisCriteriaConstraints.percentile.maximum,
			min: analysisCriteriaConstraints.percentile.minimum,
		}),
		...(minimumRangePercent
			? [
					input(minimumRangePercent, minimumRangePresets ?? [], {
						decimalScale: 10,
						label: "Candle Range (%)",
						min: marketScanCriteriaConstraints.minimumRangePercent.minimum,
						step: 0.1,
					}),
				]
			: []),
	];
}

function input(
	field: VolatilityFormField,
	presets: readonly number[],
	presentation: NumberInputProps,
): NumberInputField {
	return {
		...presentation,
		error: field.error,
		id: field.id,
		onChange: field.onChange,
		presets: presets.map((value) => ({ label: String(value), value })),
		value: field.value,
	};
}
