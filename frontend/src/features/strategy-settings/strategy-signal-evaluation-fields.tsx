import { Group } from "@mantine/core";

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
// judge a signal by.
export function StrategySignalEvaluationFields({
	disabled,
	form,
}: StrategySignalEvaluationFieldsProps) {
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
