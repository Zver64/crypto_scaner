import type { DraggableProvidedDragHandleProps } from "@hello-pangea/dnd";
import {
	Badge,
	Box,
	Button,
	Group,
	Paper,
	Stack,
	Switch,
	Text,
} from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import { IconGripVertical } from "@tabler/icons-react";
import type { ScannerIndicator } from "@/api/generated/models";
import { chartIntervalOptions } from "@/components/price-history-chart/config";
import {
	formatParameters,
	formatScale,
} from "@/features/scanner-settings/utils";

interface ScannerIndicatorRowProps {
	disabled: boolean;
	dragHandleProps: DraggableProvidedDragHandleProps | null;
	indicator: ScannerIndicator;
	onDelete(): void;
	onShowInTableChange(showInTable: boolean): void;
}

export function ScannerIndicatorRow({
	disabled,
	dragHandleProps,
	indicator,
	onDelete,
	onShowInTableChange,
}: ScannerIndicatorRowProps) {
	const used = indicator.strategies.length > 0;
	return (
		<Paper
			p="xs"
			radius="sm"
			// Strategies keep the indicators they read.
			style={
				used
					? (theme) => ({ borderColor: themeToVars(theme).colors.teal[7] })
					: undefined
			}
			withBorder
		>
			<Group align="flex-start" gap="xs" wrap="nowrap">
				<Box
					{...dragHandleProps}
					aria-label={`Move ${indicator.title}`}
					c="dimmed"
					display="flex"
					py={2}
				>
					<IconGripVertical size={18} />
				</Box>
				<Stack flex={1} gap={4}>
					<Group justify="space-between" wrap="nowrap">
						<Group gap="xs" wrap="nowrap">
							<Text fw={700}>{indicator.title}</Text>
							<Badge size="xs" variant="light">
								{chartIntervalOptions.find(
									({ value }) => value === indicator.interval,
								)?.label ?? indicator.interval}
							</Badge>
						</Group>
						<Button
							color="red"
							disabled={disabled || used}
							onClick={onDelete}
							size="compact-xs"
							variant="subtle"
						>
							Delete
						</Button>
					</Group>
					{used ? (
						<Text c="teal" size="xs">
							Used by {indicator.strategies.join(", ")}
						</Text>
					) : null}
					<Text c="dimmed" size="xs">
						{indicator.type.toUpperCase()} ·{" "}
						{formatParameters(indicator.parameters)}
					</Text>
					<Text c="dimmed" size="xs">
						{indicator.placement === "overlay"
							? "Over the candles"
							: `Pane, scale ${formatScale(indicator.scale)}`}
					</Text>
					{indicator.outputs.length === 1 ? (
						<Switch
							checked={indicator.show_in_table}
							disabled={disabled}
							label="Show in tables"
							onChange={(event) =>
								onShowInTableChange(event.currentTarget.checked)
							}
							size="xs"
						/>
					) : (
						<Text c="dimmed" size="xs">
							Outputs: {indicator.outputs.join(", ")} (chart only)
						</Text>
					)}
				</Stack>
			</Group>
		</Paper>
	);
}
