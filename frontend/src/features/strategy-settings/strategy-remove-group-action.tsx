import { ActionIcon } from "@mantine/core";
import { IconTrash } from "@tabler/icons-react";
import type { ActionProps } from "react-querybuilder";

// Removes a nested group.
export function StrategyRemoveGroupAction({
	disabled,
	handleOnClick,
	title,
}: ActionProps) {
	return (
		<ActionIcon
			aria-label={title}
			color="red"
			disabled={disabled}
			onClick={(event) => handleOnClick(event)}
			size="sm"
			variant="subtle"
		>
			<IconTrash size={14} />
		</ActionIcon>
	);
}
