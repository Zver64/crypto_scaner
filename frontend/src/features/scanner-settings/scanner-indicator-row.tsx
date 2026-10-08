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
import { useState } from "react";
import type {
	ScannerIndicator,
	ScannerIndicatorUpdate,
} from "@/api/generated/models";
import { chartIntervalOptions } from "@/features/candle-chart/config";
import { ScannerIndicatorScaleEditor } from "@/features/scanner-settings/scanner-indicator-scale-editor";
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
	const [editingScale, setEditingScale] = useState(false);
	const update = useSaveScannerIndicator(savedIndicator);
	const disabled = listDisabled || update.isPending;
	// Pending values are local to this row and fall back on failure.
	const indicator = update.isPending
		? { ...savedIndicator, ...update.variables.data }
		: savedIndicator;
	const save = (data: ScannerIndicatorUpdate) => {
		if (disabled) return;
		update.mutate({ indicatorId: indicator.id, data });
	};
	const used = indicator.strategies.length > 0;
	const display = {
		show_in_chart: indicator.show_in_chart,
		show_in_table: indicator.show_in_table,
	};
	const onDisplayChange = (
		next: Pick<ScannerIndicatorUpdate, "show_in_table" | "show_in_chart">,
	) => save({ ...next, scale: indicator.scale });
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
							onClick={() => onDelete(savedIndicator)}
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
					{indicator.placement === "overlay" ? (
						<Text c="dimmed" size="xs">
							Over the candles
						</Text>
					) : (
						<Group gap="xs" justify="space-between" wrap="nowrap">
							<Text c="dimmed" size="xs">
								Pane, scale {formatScale(indicator.scale)}
							</Text>
							{editingScale ? null : (
								<Button
									disabled={disabled}
									onClick={() => setEditingScale(true)}
									size="compact-xs"
									variant="subtle"
								>
									Edit scale
								</Button>
							)}
						</Group>
					)}
					{editingScale ? (
						<ScannerIndicatorScaleEditor
							disabled={disabled}
							onCancel={() => setEditingScale(false)}
							onSave={(scale) => {
								save({ ...display, scale });
								setEditingScale(false);
							}}
							scale={indicator.scale}
						/>
					) : null}
					{indicator.outputs.length === 1 ? null : (
						<Text c="dimmed" size="xs">
							Outputs: {indicator.outputs.join(", ")} (not a table column)
						</Text>
					)}
					<Group gap="md">
						{indicator.outputs.length === 1 ? (
							<Switch
								checked={indicator.show_in_table}
								disabled={disabled}
								label="Table"
								onChange={(event) =>
									onDisplayChange({
										...display,
										show_in_table: event.currentTarget.checked,
									})
								}
								size="xs"
							/>
						) : null}
						<Switch
							checked={indicator.show_in_chart}
							disabled={disabled}
							label="Chart"
							onChange={(event) =>
								onDisplayChange({
									...display,
									show_in_chart: event.currentTarget.checked,
								})
							}
							size="xs"
						/>
					</Group>
				</Stack>
			</Group>
		</Paper>
	);
}
