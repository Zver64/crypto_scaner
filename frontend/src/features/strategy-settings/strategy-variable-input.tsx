import { Box, Group, NativeSelect, Select } from "@mantine/core";

import {
	fieldVariable,
	variableCall,
	withInterval,
} from "@/features/strategy-settings/expressions";
import type { StrategyVariables } from "@/features/strategy-settings/types";
import {
	intervalSelectData,
	variableSelectData,
} from "@/features/strategy-settings/utils";

interface StrategyVariableInputProps {
	disabled: boolean;
	// The call or position variable the operand reads; empty while it is to
	// be chosen.
	field: string;
	label: string;
	onChange(field: string): void;
	variables: StrategyVariables;
}

// What a variable operand reads and, for a call, the period it reads it on.
// Another variable keeps the period.
export function StrategyVariableInput({
	disabled,
	field,
	label,
	onChange,
	variables,
}: StrategyVariableInputProps) {
	const { interval, key } = fieldVariable(field);
	return (
		<Group gap={6} wrap="nowrap">
			<Box flex={1} miw={0}>
				<Select
					aria-label={label}
					clearable
					data={variableSelectData(variables.variables)}
					disabled={disabled}
					// Clearing leaves the variable to be chosen again.
					onChange={(value) => {
						const variable = variables.variables.find(
							(item) => item.key === value,
						);
						const period = interval ?? variables.intervals[0];
						onChange(
							variable && period !== undefined
								? variableCall(variable, period)
								: "",
						);
					}}
					placeholder="Indicator"
					searchable
					size="sm"
					value={field === "" ? null : key}
				/>
			</Box>
			{interval === undefined ? null : (
				<NativeSelect
					aria-label={`${label} period`}
					data={intervalSelectData(variables.intervals)}
					disabled={disabled}
					onChange={(event) =>
						onChange(withInterval(field, event.currentTarget.value))
					}
					size="sm"
					value={interval}
				/>
			)}
		</Group>
	);
}
