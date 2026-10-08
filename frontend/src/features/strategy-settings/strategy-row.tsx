import {
	Badge,
	Button,
	Code,
	Group,
	Paper,
	Stack,
	Switch,
	Text,
} from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import type { Strategy } from "@/api/generated/models";
import { strategyBuysLabel } from "@/features/strategy-settings/utils";

interface StrategyRowProps {
	disabled: boolean;
	onDelete(): void;
	onEdit(): void;
	onEnabledChange(enabled: boolean): void;
	// Whether the strategy is open in the editor beside the list.
	selected?: boolean;
	strategy: Strategy;
}

export function StrategyRow({
	disabled,
	onDelete,
	onEdit,
	onEnabledChange,
	selected = false,
	strategy,
}: StrategyRowProps) {
	return (
		<Paper
			p="xs"
			radius="sm"
			style={
				selected
					? (theme) => ({
							borderColor: themeToVars(theme).colors.primaryColors.filled,
						})
					: undefined
			}
			withBorder
		>
			<Stack gap={6}>
				<Group justify="space-between" wrap="nowrap">
					<Group gap="xs" miw={0} wrap="nowrap">
						<Text fw={700} truncate>
							{strategy.name}
						</Text>
						{strategy.valid ? null : (
							<Badge color="red" size="xs" variant="light">
								Invalid
							</Badge>
						)}
					</Group>
					<Switch
						aria-label={`Evaluate ${strategy.name}`}
						checked={strategy.enabled}
						disabled={disabled}
						onChange={(event) => onEnabledChange(event.currentTarget.checked)}
						size="sm"
					/>
				</Group>
				{strategy.problem === undefined ? null : (
					<Text c="red" size="xs" style={{ whiteSpace: "pre-wrap" }}>
						{strategy.problem}
					</Text>
				)}
				<Code block>{strategy.expression}</Code>
				{[
					{ expression: strategy.exit_expression, title: "Exit" },
					{ expression: strategy.take_profit_expression, title: "Take profit" },
					{ expression: strategy.stop_loss_expression, title: "Stop loss" },
				].map(({ expression, title }) =>
					expression === "" ? null : (
						<Stack gap={2} key={title}>
							<Text c="dimmed" size="xs">
								{title}
							</Text>
							<Code block>{expression}</Code>
						</Stack>
					),
				)}
				<Text c="dimmed" size="xs">
					{strategyBuysLabel(strategy)}
				</Text>
				{strategy.message === "" ? null : (
					<Text c="dimmed" size="xs" style={{ whiteSpace: "pre-wrap" }}>
						{strategy.message}
					</Text>
				)}
				<Group gap="xs" justify="flex-end">
					<Button
						disabled={disabled}
						onClick={onEdit}
						size="compact-xs"
						variant="subtle"
					>
						Edit
					</Button>
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
			</Stack>
		</Paper>
	);
}
