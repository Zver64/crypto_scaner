import { NativeSelect, Stack } from "@mantine/core";
import type { ReactNode } from "react";

import type { IndicatorParameter } from "@/api/generated/models";
import { indicatorSourceItem } from "@/features/strategy-settings/constants";
import { NestedIndicatorInput } from "@/features/strategy-settings/nested-indicator-input";
import type {
	IndicatorArgument,
	StrategyIndicatorVariable,
} from "@/features/strategy-settings/types";
import { choiceSelectData } from "@/features/strategy-settings/utils";

interface IndicatorSourceInputProps {
	arg: IndicatorArgument;
	depth: number;
	disabled: boolean;
	// The period of the call, which an indicator read as the source reads too.
	interval: string;
	// Marks a source whose indicator is still to be chosen.
	invalid: boolean;
	onChange(arg: IndicatorArgument): void;
	parameter: IndicatorParameter;
	// The parameter editor of the indicator read as the source.
	parameters: ReactNode;
	// The indicators the source may read; undefined offers only its choices.
	sources?: StrategyIndicatorVariable[];
}

// The series an indicator reads: one of the choices of its source
// parameter, such as a candle field, or another indicator on the same
// period, edited below it.
export function IndicatorSourceInput({
	arg,
	depth,
	disabled,
	interval,
	invalid,
	onChange,
	parameter,
	parameters,
	sources = [],
}: IndicatorSourceInputProps) {
	return (
		<Stack gap={6}>
			<NativeSelect
				aria-label={parameter.title}
				data={[
					...choiceSelectData(parameter),
					...(sources.length > 0 || arg === indicatorSourceItem.value
						? [indicatorSourceItem]
						: []),
				]}
				disabled={disabled}
				// Another indicator starts with none chosen.
				onChange={(event) => onChange(event.currentTarget.value)}
				size="sm"
				value={typeof arg === "string" ? arg : indicatorSourceItem.value}
			/>
			{typeof arg === "string" && arg !== indicatorSourceItem.value ? null : (
				<NestedIndicatorInput
					call={typeof arg === "string" ? undefined : arg}
					depth={depth}
					disabled={disabled}
					interval={interval}
					invalid={invalid}
					label={parameter.title}
					onChange={onChange}
					parameters={parameters}
					sources={sources}
				/>
			)}
		</Stack>
	);
}
