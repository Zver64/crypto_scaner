import { Button } from "@mantine/core";
import type { ActionProps } from "react-querybuilder";

// Adds a condition or a group to a group.
export function StrategyAddAction({
	disabled,
	handleOnClick,
	label,
}: ActionProps) {
	return (
		<Button
			disabled={disabled}
			onClick={(event) => handleOnClick(event)}
			size="compact-xs"
			variant="light"
		>
			{label}
		</Button>
	);
}
