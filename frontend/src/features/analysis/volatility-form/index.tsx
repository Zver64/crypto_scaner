import type { NumberInputProps } from "@mantine/core";
import { NumberInputFieldset } from "@/components/number-input-fieldset";
import {
	analysisCriteriaConstraints,
	maximumPeriodForUnit,
} from "@/features/analysis/criteria";
import type {
	VolatilityFieldsetProps,
	VolatilityFormField,
} from "@/features/analysis/volatility-form/types";
import { marketScanCriteriaConstraints } from "@/features/market-scan/criteria";

export function VolatilityFieldset({
	minimumRangePercent,
	minimumRangePresets,
	percentile,
	percentilePresets,
	period,
	periodPresets,
	size,
	title,
	unit,
}: VolatilityFieldsetProps) {
	return (
		<NumberInputFieldset
			inputs={[
				input(period, periodPresets, {
					allowDecimal: false,
					label: "Period",
					max: maximumPeriodForUnit(unit),
					min: analysisCriteriaConstraints.period.minimum,
					size,
				}),
				input(percentile, percentilePresets, {
					allowDecimal: false,
					label: "Percentile",
					max: analysisCriteriaConstraints.percentile.maximum,
					min: analysisCriteriaConstraints.percentile.minimum,
					size,
				}),
				...(minimumRangePercent
					? [
							input(minimumRangePercent, minimumRangePresets ?? [], {
								decimalScale: 10,
								label: "Candle Range (%)",
								min: marketScanCriteriaConstraints.minimumRangePercent.minimum,
								size,
								step: 0.1,
							}),
						]
					: []),
			]}
			title={title}
		/>
	);
}

function input(
	field: VolatilityFormField,
	presets: readonly number[],
	presentation: NumberInputProps,
) {
	return {
		...presentation,
		error: field.error,
		id: field.id,
		onChange: field.onChange,
		presets: presets.map((value) => ({ label: String(value), value })),
		value: field.value,
	};
}
