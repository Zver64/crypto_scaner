import { NumberInput, Select } from "@mantine/core";
import type { IndicatorParameter } from "@/api/generated/models";

interface ScannerIndicatorParameterFieldProps {
	onChange(value: number | string): void;
	parameter: IndicatorParameter;
	value: number | string | undefined;
}

export function ScannerIndicatorParameterField({
	onChange,
	parameter,
	value,
}: ScannerIndicatorParameterFieldProps) {
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
				onChange={(next) => onChange(next === null ? "" : Number(next))}
				value={value === undefined || value === "" ? null : String(value)}
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
			onChange={onChange}
			placeholder={String(parameter.default)}
			value={value ?? ""}
		/>
	);
}
