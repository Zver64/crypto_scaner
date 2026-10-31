import { ActionIcon } from "@mantine/core";
import { IconMagnet } from "@tabler/icons-react";

interface AlignBoundButtonProps {
	// Null when the grid is already even or the price cannot move.
	onAlign: (() => void) | null;
}

// Moves a range price to where the grid step equals the selected one.
export function AlignBoundButton({ onAlign }: AlignBoundButtonProps) {
	return (
		<ActionIcon
			disabled={!onAlign}
			onClick={onAlign ?? undefined}
			size="sm"
			variant="subtle"
		>
			<IconMagnet size={16} />
		</ActionIcon>
	);
}
