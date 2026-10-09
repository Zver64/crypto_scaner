import { Group, NativeSelect } from "@mantine/core";
import type { SignalTargetRatio, SignalWindow } from "@/api/generated/models";
import {
	signalTargetRatios,
	signalWindows,
} from "@/features/strategy-settings/constants";
import { formatNumber } from "@/utils/number-format";

interface StrategySignalEvaluationFieldsProps {
	disabled: boolean;
	onTargetRatioChange(value: SignalTargetRatio): void;
	onWindowChange(value: SignalWindow): void;
	targetRatio: SignalTargetRatio;
	window: SignalWindow;
}

// The target distance in stops and the window in candles that backtests
// judge a signal by.
export function StrategySignalEvaluationFields({
	disabled,
	onTargetRatioChange,
	onWindowChange,
	targetRatio,
	window,
}: StrategySignalEvaluationFieldsProps) {
	return (
		<Group grow>
			<NativeSelect
				data={signalTargetRatios.map((value) => ({
					label: formatNumber(value),
					value: String(value),
				}))}
				disabled={disabled}
				label="Target, stops"
				onChange={(event) => {
					const next = signalTargetRatios.find(
						(value) => String(value) === event.currentTarget.value,
					);
					if (next !== undefined) onTargetRatioChange(next);
				}}
				value={String(targetRatio)}
			/>
			<NativeSelect
				data={signalWindows.map((value) => ({
					label: formatNumber(value),
					value: String(value),
				}))}
				disabled={disabled}
				label="Window, candles"
				onChange={(event) => {
					const next = signalWindows.find(
						(value) => String(value) === event.currentTarget.value,
					);
					if (next !== undefined) onWindowChange(next);
				}}
				value={String(window)}
			/>
		</Group>
	);
}
