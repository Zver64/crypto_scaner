import { Text, Tooltip, UnstyledButton } from "@mantine/core";
import { useClipboard } from "@mantine/hooks";

import type { StrategyID } from "@/api/generated/models";
import { shortId } from "@/utils/short-id";

interface StrategyIdProps {
	id: StrategyID;
}

// The end of the id; a tap copies the whole id.
export function StrategyId({ id }: StrategyIdProps) {
	const clipboard = useClipboard({ timeout: 1500 });
	return (
		<Tooltip label="Copied" opened={clipboard.copied}>
			<UnstyledButton onClick={() => clipboard.copy(id)}>
				<Text c="dimmed" ff="monospace" size="xs">
					#{shortId(id)}
				</Text>
			</UnstyledButton>
		</Tooltip>
	);
}
