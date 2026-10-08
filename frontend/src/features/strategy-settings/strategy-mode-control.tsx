import { SegmentedControl, Stack } from "@mantine/core";
import { SignalDirection } from "@/api/generated/models";
import { signalDirectionLabels } from "@/features/strategy-settings/constants";
import type { StrategyMode } from "@/features/strategy-settings/types";

interface StrategyModeControlProps {
	direction: SignalDirection;
	disabled: boolean;
	mode: StrategyMode;
	onDirectionChange(direction: SignalDirection): void;
	onModeChange(mode: StrategyMode): void;
}

const modeOptions = [
	{ label: "Signal", value: "signal" },
	{ label: "Strategy", value: "strategy" },
] satisfies { label: string; value: StrategyMode }[];

const directionOptions = Object.values(SignalDirection).map((value) => ({
	label: signalDirectionLabels[value],
	value,
}));

// Chooses between a signal, which only announces the move it expects, and a
// strategy that trades; a signal also picks its move.
export function StrategyModeControl({
	direction,
	disabled,
	mode,
	onDirectionChange,
	onModeChange,
}: StrategyModeControlProps) {
	return (
		<Stack gap="xs">
			<SegmentedControl<StrategyMode>
				data={modeOptions}
				disabled={disabled}
				fullWidth
				onChange={onModeChange}
				value={mode}
			/>
			{mode === "signal" ? (
				<SegmentedControl<SignalDirection>
					data={directionOptions}
					disabled={disabled}
					fullWidth
					onChange={onDirectionChange}
					value={direction}
				/>
			) : null}
		</Stack>
	);
}
