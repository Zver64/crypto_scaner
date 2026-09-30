import {
	Badge,
	Button,
	Checkbox,
	Group,
	Paper,
	Stack,
	Text,
} from "@mantine/core";
import type { User } from "@/api/generated/models";
import { formatUserName } from "@/features/user-settings/utils";

interface UserRowProps {
	disabled: boolean;
	onDelete(): void;
	onStrategyAlertsChange(enabled: boolean): void;
	user: User;
}

export function UserRow({
	disabled,
	onDelete,
	onStrategyAlertsChange,
	user,
}: UserRowProps) {
	return (
		<Paper p="xs" radius="sm" withBorder>
			<Group justify="space-between" wrap="nowrap">
				<Stack gap={2} miw={0}>
					<Group gap="xs" wrap="nowrap">
						<Text fw={700} truncate>
							{formatUserName(user)}
						</Text>
						{user.administrator ? (
							<Badge size="xs" variant="light">
								Administrator
							</Badge>
						) : null}
					</Group>
					<Text c="dimmed" size="xs">
						{user.username ? `@${user.username} · ` : ""}ID {user.telegram_id}
					</Text>
					<Checkbox
						checked={user.strategy_alerts}
						// The administrator always receives strategy alerts.
						disabled={disabled || user.administrator}
						label="Strategy alerts"
						mt={4}
						onChange={(event) =>
							onStrategyAlertsChange(event.currentTarget.checked)
						}
						size="xs"
					/>
				</Stack>
				{user.administrator ? null : (
					<Button
						color="red"
						disabled={disabled}
						onClick={onDelete}
						size="compact-xs"
						variant="subtle"
					>
						Delete
					</Button>
				)}
			</Group>
		</Paper>
	);
}
