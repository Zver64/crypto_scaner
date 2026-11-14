import { SegmentedControl } from "@mantine/core";
import {
	type CombinatorSelectorProps,
	toFlatOptionArray,
} from "react-querybuilder";

// Whether a group needs all or any of its conditions.
export function StrategyCombinatorSelector({
	disabled,
	handleOnChange,
	options,
	value,
}: CombinatorSelectorProps) {
	return (
		<SegmentedControl
			data={toFlatOptionArray(options).map(({ label, value }) => ({
				label,
				value,
			}))}
			disabled={disabled}
			onChange={handleOnChange}
			size="xs"
			value={value}
		/>
	);
}
