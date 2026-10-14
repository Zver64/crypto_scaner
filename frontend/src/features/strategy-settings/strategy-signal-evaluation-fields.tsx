import { Group, NativeSelect } from "@mantine/core";
import type { UseFormReturnType } from "@mantine/form";

import {
	signalTargetRatios,
	signalWindows,
} from "@/features/strategy-settings/constants";
import type { StrategyFormValues } from "@/features/strategy-settings/types";
import { formatNumber } from "@/utils/number-format";

interface StrategySignalEvaluationFieldsProps {
	disabled: boolean;
	form: UseFormReturnType<StrategyFormValues>;
}

// The target distance in stops and the window in candles that backtests
// judge a signal by.
export function StrategySignalEvaluationFields({
	disabled,
	form,
}: StrategySignalEvaluationFieldsProps) {
	return (
		<Group grow>
			<NativeSelect
				data={signalTargetRatios.map((value) => ({
					label: formatNumber(value),
					value: String(value),
				}))}
				{...form.getInputProps("targetRatio")}
				disabled={disabled}
				key={form.key("targetRatio")}
				label="Target, stops"
			/>
			<NativeSelect
				data={signalWindows.map((value) => ({
					label: formatNumber(value),
					value: String(value),
				}))}
				{...form.getInputProps("window")}
				disabled={disabled}
				key={form.key("window")}
				label="Window, candles"
			/>
		</Group>
	);
}
