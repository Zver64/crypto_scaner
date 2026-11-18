import { Select, Stack } from "@mantine/core";
import type { ReactNode } from "react";

import { defaultIndicatorCall } from "@/features/strategy-settings/expressions";
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
	label: string;
	onChange(call: IndicatorCallParts): void;
	// The parameter editor of call, shown below it.
	parameters: ReactNode;
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
	label,
	onChange,
	parameters,
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
				<NestingLine depth={depth + 1}>{parameters}</NestingLine>
			) : null}
		</Stack>
	);
}
