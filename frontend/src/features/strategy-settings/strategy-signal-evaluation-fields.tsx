import { Group } from "@mantine/core";
import { useSelector } from "@tanstack/react-form";

import { Direction } from "@/api/generated/models";
import {
	signalTargetRatios,
	signalWindows,
} from "@/features/strategy-settings/constants";
import type { StrategyFormApi } from "@/features/strategy-settings/types";
import { formatNumber } from "@/utils/number-format";

interface StrategySignalEvaluationFieldsProps {
	disabled: boolean;
	form: StrategyFormApi;
}

// The target distance in stops and the window in candles that backtests
// judge a signal by; none for a simple signal, which is never judged.
export function StrategySignalEvaluationFields({
	disabled,
	form,
}: StrategySignalEvaluationFieldsProps) {
	const simple = useSelector(
		form.store,
		({ values }) => values.direction === Direction.simple,
	);
	if (simple) {
		return null;
	}
	return (
		<Group grow>
			<form.AppField name="targetRatio">
				{(field) => (
					<field.NativeSelectField
						data={signalTargetRatios.map((value) => ({
							label: formatNumber(value),
							value: String(value),
						}))}
						disabled={disabled}
						label="Target, stops"
					/>
				)}
			</form.AppField>
			<form.AppField name="window">
				{(field) => (
					<field.NativeSelectField
						data={signalWindows.map((value) => ({
							label: formatNumber(value),
							value: String(value),
						}))}
						disabled={disabled}
						label="Window, candles"
					/>
				)}
			</form.AppField>
		</Group>
	);
}
