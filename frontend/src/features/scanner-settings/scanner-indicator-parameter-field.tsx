import { NumberInput, Select } from "@mantine/core";
import type { UseFormReturnType } from "@mantine/form";

import type { IndicatorParameter } from "@/api/generated/models";
import type { ScannerIndicatorDraft } from "@/features/scanner-settings/types";

interface ScannerIndicatorParameterFieldProps {
	form: UseFormReturnType<ScannerIndicatorDraft>;
	parameter: IndicatorParameter;
}

export function ScannerIndicatorParameterField({
	form,
	parameter,
}: ScannerIndicatorParameterFieldProps) {
	const path = `parameters.${parameter.key}`;
	if (parameter.kind === "choice") {
		return (
			<Select
				allowDeselect={false}
				data={(parameter.choices ?? []).map((choice) => ({
					label: choice.title,
					value: String(choice.value),
				}))}
				description={parameter.description}
				label={parameter.title}
				{...form.getInputProps(path)}
			/>
		);
	}
	return (
		<NumberInput
			allowDecimal={parameter.kind === "real"}
			description={parameter.description}
			label={parameter.title}
			max={parameter.maximum}
			min={parameter.minimum}
			placeholder={String(parameter.default)}
			{...form.getInputProps(path)}
		/>
	);
}
