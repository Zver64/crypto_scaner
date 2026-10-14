import { SegmentedControl } from "@mantine/core";

import { Direction } from "@/api/generated/models";
import {
	directionLabels,
	strategyDirections,
} from "@/features/strategy-settings/constants";
import type {
	StrategyFormApi,
	StrategyKind,
} from "@/features/strategy-settings/types";

interface StrategyDirectionControlProps {
	disabled: boolean;
	form: StrategyFormApi;
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
		<form.Field
			listeners={{
				// Take profit and stop loss are written for one side. Listeners do
				// not run when an opened draft resets the form, so its saved prices
				// stay.
				onChange: () => {
					form.setFieldValue("takeProfit", {});
					form.setFieldValue("stopLoss", {});
				},
			}}
			name="direction"
		>
			{(field) => (
				<SegmentedControl<Direction>
					data={directionOptions[kind]}
					disabled={disabled}
					fullWidth
					onChange={field.handleChange}
					value={field.state.value}
				/>
			)}
		</form.Field>
	);
}
