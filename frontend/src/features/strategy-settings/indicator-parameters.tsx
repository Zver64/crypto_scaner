import { NativeSelect, NumberInput, Stack, Text } from "@mantine/core";

import { IndicatorParameterKind } from "@/api/generated/models";
import {
	canonicalArgument,
	canonicalNumber,
} from "@/features/strategy-settings/expressions";
import { IndicatorSourceInput } from "@/features/strategy-settings/indicator-source-input";
import type {
	IndicatorArgument,
	IndicatorCallParts,
	StrategyIndicatorVariable,
} from "@/features/strategy-settings/types";
import {
	choiceSelectData,
	issuePathsUnder,
} from "@/features/strategy-settings/utils";

interface IndicatorParametersProps {
	call: IndicatorCallParts;
	// How many groups and functions enclose the call, which picks the line
	// color of an indicator read as a source.
	depth: number;
	disabled: boolean;
	// The paths of the validation issues of the call's arguments, each
	// starting with the position of the argument.
	issuePaths: readonly PropertyKey[][];
	onChange(call: IndicatorCallParts): void;
	// The indicators a source may read; undefined for an indicator that is a
	// source itself, whose own source is a candle field.
	sources?: StrategyIndicatorVariable[];
}

// The parameters of an indicator call, each under its title: a number, a
// choice, or the source the indicator reads.
export function IndicatorParameters({
	call,
	depth,
	disabled,
	issuePaths,
	onChange,
	sources,
}: IndicatorParametersProps) {
	const replace = (index: number, arg: IndicatorArgument) =>
		onChange({
			...call,
			args: call.args.map((old, i) => (i === index ? arg : old)),
		});
	return (
		<Stack gap="xs">
			{call.variable.indicator.parameters.map((parameter, index) => {
				const arg = call.args[index] ?? "";
				const paths = issuePathsUnder(issuePaths, index);
				const invalid = paths.some((path) => path.length === 0);
				return (
					<Stack gap={2} key={parameter.key}>
						<Text c="dimmed" size="xs">
							{parameter.title}
						</Text>
						{parameter.kind === IndicatorParameterKind.source ? (
							<IndicatorSourceInput
								arg={arg}
								depth={depth}
								disabled={disabled}
								interval={call.interval}
								invalid={invalid}
								onChange={(next) => replace(index, next)}
								parameter={parameter}
								parameters={
									typeof arg === "string" ? null : (
										<IndicatorParameters
											call={arg}
											depth={depth + 1}
											disabled={disabled}
											issuePaths={paths}
											onChange={(next) => replace(index, next)}
										/>
									)
								}
								sources={sources}
							/>
						) : typeof arg !== "string" ? null : parameter.kind ===
						  IndicatorParameterKind.choice ? (
							<NativeSelect
								aria-label={parameter.title}
								data={choiceSelectData(parameter)}
								disabled={disabled}
								error={invalid}
								onChange={(event) => replace(index, event.currentTarget.value)}
								size="sm"
								value={arg}
							/>
						) : (
							<NumberInput
								allowDecimal={parameter.kind === IndicatorParameterKind.real}
								allowNegative={(parameter.minimum ?? 0) < 0}
								aria-label={parameter.title}
								disabled={disabled}
								error={invalid}
								max={parameter.maximum}
								min={parameter.minimum}
								// The call holds typed text, such as "2." or "2.0", while
								// the input is focused, so the decimals can be typed,
								// and the canonical number once it is left.
								onBlur={() => {
									const canonical = canonicalArgument(arg);
									if (canonical !== arg) replace(index, canonical);
								}}
								onChange={(value) =>
									replace(
										index,
										typeof value === "number" ? canonicalNumber(value) : value,
									)
								}
								size="sm"
								value={arg}
							/>
						)}
					</Stack>
				);
			})}
		</Stack>
	);
}
