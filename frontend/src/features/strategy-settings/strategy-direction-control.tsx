import { SegmentedControl } from "@mantine/core";
import { Direction } from "@/api/generated/models";
import {
	directionLabels,
	strategyDirections,
} from "@/features/strategy-settings/constants";
import type { StrategyKind } from "@/features/strategy-settings/types";

interface StrategyDirectionControlProps {
	direction: Direction;
	disabled: boolean;
	kind: StrategyKind;
	onChange(direction: Direction): void;
}

// The directions a signal or a strategy offers.
const directionOptions = {
	signal: Object.values(Direction).map((value) => ({
		label: directionLabels[value],
		value,
	})),
	strategy: strategyDirections.map((value) => ({
		label: directionLabels[value],
		value,
	})),
} satisfies Record<StrategyKind, { label: string; value: Direction }[]>;

// Picks the move a signal expects or the direction a strategy trades.
export function StrategyDirectionControl({
	direction,
	disabled,
	kind,
	onChange,
}: StrategyDirectionControlProps) {
	return (
		<SegmentedControl<Direction>
			data={directionOptions[kind]}
			disabled={disabled}
			fullWidth
			onChange={onChange}
			value={direction}
		/>
	);
}
