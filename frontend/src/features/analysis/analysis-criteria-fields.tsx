import { NumberInput, type NumberInputProps } from "@mantine/core";
import { marketScanCriteriaConstraints } from "../market-scan/criteria";

interface AnalysisCriteriaFieldsProps {
	percentileInputProps: NumberInputProps;
	percentileKey: string;
	periodInputProps: NumberInputProps;
	periodKey: string;
}

export function AnalysisCriteriaFields({
	percentileInputProps,
	percentileKey,
	periodInputProps,
	periodKey,
}: AnalysisCriteriaFieldsProps) {
	return (
		<>
			<NumberInput
				allowDecimal={false}
				key={periodKey}
				label="Analysis Period"
				max={marketScanCriteriaConstraints.periodDays.maximum}
				min={marketScanCriteriaConstraints.periodDays.minimum}
				suffix=" days"
				{...periodInputProps}
			/>
			<NumberInput
				allowDecimal={false}
				key={percentileKey}
				label="Range Percentile"
				max={marketScanCriteriaConstraints.percentile.maximum}
				min={marketScanCriteriaConstraints.percentile.minimum}
				{...percentileInputProps}
			/>
		</>
	);
}
