import { Switch } from "@mantine/core";
import type { NotToggleProps } from "react-querybuilder";

// Negates a group.
export function StrategyNotToggle({
	checked,
	disabled,
	handleOnChange,
	label,
}: NotToggleProps) {
	return (
		<Switch
			checked={Boolean(checked)}
			disabled={disabled}
			label={label}
			onChange={(event) => handleOnChange(event.currentTarget.checked)}
			size="xs"
		/>
	);
}
