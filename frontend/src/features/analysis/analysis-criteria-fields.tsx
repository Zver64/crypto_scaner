import {
	NumberInput,
	type NumberInputProps,
	SegmentedControl,
} from "@mantine/core";
import {
	type AnalysisUnit,
	analysisCriteriaConstraints,
	maximumPeriodForUnit,
} from "./criteria";

interface AnalysisCriteriaFieldsProps {
	percentileInputProps: NumberInputProps;
	percentileKey: string;
	periodInputProps: NumberInputProps;
	periodKey: string;
	unit: AnalysisUnit;
	onUnitChange(unit: AnalysisUnit): void;
}

export function AnalysisCriteriaFields({
	percentileInputProps,
	percentileKey,
	periodInputProps,
	periodKey,
	unit,
	onUnitChange,
}: AnalysisCriteriaFieldsProps) {
	return (
		<>
			<NumberInput
				allowDecimal={false}
				key={periodKey}
				label={`Analysis Period (${unit})`}
				max={maximumPeriodForUnit(unit)}
				min={analysisCriteriaConstraints.period.minimum}
				suffix={` ${unit}`}
				{...periodInputProps}
			/>
			<SegmentedControl
				data={[
					{ label: "Days", value: "days" },
					{ label: "Hours", value: "hours" },
				]}
				value={unit}
				onChange={(value) => onUnitChange(value as AnalysisUnit)}
			/>
			<NumberInput
				allowDecimal={false}
				key={percentileKey}
				label="Range Percentile"
				max={analysisCriteriaConstraints.percentile.maximum}
				min={analysisCriteriaConstraints.percentile.minimum}
				{...percentileInputProps}
			/>
		</>
	);
}
