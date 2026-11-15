import { Select, Stack } from "@mantine/core";

import { defaultIndicatorCall } from "@/features/strategy-settings/expressions";
import { IndicatorParameters } from "@/features/strategy-settings/indicator-parameters";
import { NestingLine } from "@/features/strategy-settings/nesting-line";
import type {
	IndicatorCallParts,
	StrategyIndicatorVariable,
} from "@/features/strategy-settings/types";
import { variableSelectData } from "@/features/strategy-settings/utils";

interface NestedIndicatorInputProps {
	// Undefined while the indicator is to be chosen.
	call: IndicatorCallParts | undefined;
	depth: number;
	disabled: boolean;
	// The period of the call the indicator is the source of.
	interval: string;
	invalid: boolean;
	issuePaths: readonly PropertyKey[][];
	label: string;
	onChange(call: IndicatorCallParts): void;
	// The indicators the source may read.
	sources: StrategyIndicatorVariable[];
}

// An indicator read as the source of another: chosen without a period,
// since it reads that of the other, with its parameters below, its own
// source a candle field.
export function NestedIndicatorInput({
	call,
	depth,
	disabled,
	interval,
	invalid,
	issuePaths,
	label,
	onChange,
	sources,
}: NestedIndicatorInputProps) {
	return (
		<Stack gap={6}>
			<Select
				aria-label={`${label} indicator`}
				data={variableSelectData(sources)}
				disabled={disabled}
				error={invalid}
				onChange={(value) => {
					const variable = sources.find(({ name }) => name === value);
					if (variable) onChange(defaultIndicatorCall(variable, interval));
				}}
				placeholder="Indicator"
				searchable
				size="sm"
				value={call?.variable.name ?? null}
			/>
			{call && call.variable.indicator.parameters.length > 0 ? (
				<NestingLine depth={depth + 1}>
					<IndicatorParameters
						call={call}
						depth={depth + 1}
						disabled={disabled}
						issuePaths={issuePaths}
						onChange={onChange}
					/>
				</NestingLine>
			) : null}
		</Stack>
	);
}
