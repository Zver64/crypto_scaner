import { ActionIcon } from "@mantine/core";
import { IconX } from "@tabler/icons-react";
import type { ActionProps } from "react-querybuilder";

// Removes a condition.
export function StrategyRemoveRuleAction({
	disabled,
	handleOnClick,
}: ActionProps) {
	return (
		<ActionIcon
			aria-label="Remove condition"
			size="input-sm"
			disabled={disabled}
			onClick={(event) => handleOnClick(event)}
			variant="default"
		>
			<IconX size={18} />
		</ActionIcon>
	);
}
