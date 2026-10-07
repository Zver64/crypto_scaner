import { Button, Group, Paper, Stack, Text } from "@mantine/core";
import type { ApiToken } from "@/api/generated/models";
import { formatDateTime } from "@/utils/date-time-format";

interface ApiTokenRowProps {
	disabled: boolean;
	onDelete(): void;
	token: ApiToken;
}

export function ApiTokenRow({ disabled, onDelete, token }: ApiTokenRowProps) {
	return (
		<Paper p="xs" radius="sm" withBorder>
			<Group justify="space-between" wrap="nowrap">
				<Stack gap={2} miw={0}>
					<Text fw={700} truncate>
						{token.name}
					</Text>
					<Text c="dimmed" size="xs">
						Created {formatDateTime(token.created_at)} · Last used{" "}
						{token.last_used_at ? formatDateTime(token.last_used_at) : "never"}
					</Text>
				</Stack>
				<Button
					color="red"
					disabled={disabled}
					onClick={onDelete}
					size="compact-xs"
					variant="subtle"
				>
					Delete
				</Button>
			</Group>
		</Paper>
	);
}
