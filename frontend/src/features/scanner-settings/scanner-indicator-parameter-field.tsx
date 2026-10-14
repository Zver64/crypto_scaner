import type { IndicatorParameter } from "@/api/generated/models";
import { NumberField } from "@/components/form/number-field";
import { SelectField } from "@/components/form/select-field";

interface ScannerIndicatorParameterFieldProps {
	parameter: IndicatorParameter;
}

// The input of one indicator parameter, rendered inside its form field.
export function ScannerIndicatorParameterField({
	parameter,
}: ScannerIndicatorParameterFieldProps) {
	if (parameter.kind === "choice") {
		return (
			<SelectField
				allowDeselect={false}
				data={(parameter.choices ?? []).map((choice) => ({
					label: choice.title,
					value: String(choice.value),
				}))}
				description={parameter.description}
				label={parameter.title}
			/>
		);
	}
	return (
		<NumberField
			allowDecimal={parameter.kind === "real"}
			description={parameter.description}
			label={parameter.title}
			max={parameter.maximum}
			min={parameter.minimum}
			placeholder={String(parameter.default)}
		/>
	);
}
