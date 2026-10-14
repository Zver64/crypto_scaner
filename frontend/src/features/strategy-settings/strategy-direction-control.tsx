import { SegmentedControl } from "@mantine/core";
import type { UseFormReturnType } from "@mantine/form";
import { Direction } from "@/api/generated/models";
import {
	directionLabels,
	strategyDirections,
} from "@/features/strategy-settings/constants";
import type {
	StrategyFormValues,
	StrategyKind,
} from "@/features/strategy-settings/types";

interface StrategyDirectionControlProps {
	disabled: boolean;
	form: UseFormReturnType<StrategyFormValues>;
	kind: StrategyKind;
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
	disabled,
	form,
	kind,
}: StrategyDirectionControlProps) {
	return (
		<SegmentedControl<Direction>
			data={directionOptions[kind]}
			defaultValue={form.getValues().direction}
			disabled={disabled}
			fullWidth
			// Not getInputProps: a direction change also clears the prices.
			// @mantine/form has no field-change hook for this: onValuesChange
			// also runs when an opened draft resets the form, and would clear
			// its saved prices.
			onChange={(direction) => {
				form.setFieldValue("direction", direction, { forceUpdate: false });
				// Take profit and stop loss are written for one side.
				form.setFieldValue("takeProfit", {}, { forceUpdate: false });
				form.setFieldValue("stopLoss", {}, { forceUpdate: false });
			}}
		/>
	);
}
