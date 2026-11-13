import type { DraggableProvidedDragHandleProps } from "@hello-pangea/dnd";
import { Badge, Box, Button, Group, Paper, Stack, Text } from "@mantine/core";
import { IconGripVertical } from "@tabler/icons-react";
import { useState } from "react";

import type { ScannerIndicator } from "@/api/generated/models";
import { chartIntervalOptions } from "@/features/candle-chart/config";
import { ScannerIndicatorSettingsEditor } from "@/features/scanner-settings/scanner-indicator-settings-editor";
import { useSaveScannerIndicator } from "@/features/scanner-settings/use-save-scanner-indicator";
import {
	formatParameters,
	formatScale,
} from "@/features/scanner-settings/utils";

interface ScannerIndicatorRowProps {
	disabled: boolean;
	dragHandleProps: DraggableProvidedDragHandleProps | null;
	indicator: ScannerIndicator;
	onDelete(indicator: ScannerIndicator): void;
}

export function ScannerIndicatorRow({
	disabled: listDisabled,
	dragHandleProps,
	indicator: savedIndicator,
	onDelete,
}: ScannerIndicatorRowProps) {
	const [editing, setEditing] = useState(false);
	const update = useSaveScannerIndicator(savedIndicator);
	const disabled = listDisabled || update.isPending;
	// Pending values are local to this row and fall back on failure.
	const indicator = update.isPending
		? { ...savedIndicator, ...update.variables.data }
		: savedIndicator;
	return (
		<Paper p="xs" radius="sm" withBorder>
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
						<Text fw={700}>{indicator.title}</Text>
						<Group gap={4} wrap="nowrap">
							{editing ? null : (
								<Button
									disabled={disabled}
									onClick={() => setEditing(true)}
									size="compact-xs"
									variant="subtle"
								>
									Edit
								</Button>
							)}
							<Button
								color="red"
								disabled={disabled}
								onClick={() => onDelete(savedIndicator)}
								size="compact-xs"
								variant="subtle"
							>
								Delete
							</Button>
						</Group>
					</Group>
					{indicator.periods.length === 0 ? null : (
						<Group gap={4}>
							{indicator.periods.map((period) => (
								<Badge key={period.interval} size="xs" variant="light">
									{[
										chartIntervalOptions.find(
											({ value }) => value === period.interval,
										)?.label ?? period.interval,
										...(period.show_in_table ? ["Table"] : []),
										...(period.show_in_chart ? ["Chart"] : []),
									].join(" · ")}
								</Badge>
							))}
						</Group>
					)}
					<Text c="dimmed" size="xs">
						{indicator.type.toUpperCase()} ·{" "}
						{formatParameters(indicator.parameters)}
					</Text>
					<Text c="dimmed" size="xs">
						{indicator.placement === "pane"
							? `Pane, scale ${formatScale(indicator.scale)}`
							: indicator.placement === "volume"
								? "Over the volumes"
								: "Over the candles"}
					</Text>
					{indicator.outputs.length === 1 ? null : (
						<Text c="dimmed" size="xs">
							Outputs: {indicator.outputs.join(", ")} (not a table column)
						</Text>
					)}
					{editing ? (
						<ScannerIndicatorSettingsEditor
							disabled={disabled}
							indicator={indicator}
							onCancel={() => setEditing(false)}
							onSave={(data) => {
								if (disabled) return;
								update.mutate({ data, indicatorId: indicator.id });
								setEditing(false);
							}}
						/>
					) : null}
				</Stack>
			</Group>
		</Paper>
	);
}
