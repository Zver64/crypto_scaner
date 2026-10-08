import { DragDropContext, Draggable, Droppable } from "@hello-pangea/dnd";
import { Box, Text } from "@mantine/core";
import type { ScannerIndicator } from "@/api/generated/models";
import { ScannerIndicatorRow } from "@/features/scanner-settings/scanner-indicator-row";
import { moveIndicator } from "@/features/scanner-settings/utils";

interface ScannerIndicatorListProps {
	disabled: boolean;
	indicators: readonly ScannerIndicator[];
	onDelete(indicator: ScannerIndicator): void;
	onReorder(ids: number[]): void;
}

// The configured indicators in display order, which orders the table columns
// and the chart indicators. Rows are reordered by dragging their handle.
export function ScannerIndicatorList({
	disabled,
	indicators,
	onDelete,
	onReorder,
}: ScannerIndicatorListProps) {
	if (indicators.length === 0) {
		return (
			<Text c="dimmed" size="sm">
				No indicators are configured. Charts and tables show none.
			</Text>
		);
	}
	return (
		<DragDropContext
			onDragEnd={({ destination, source }) => {
				if (!destination || destination.index === source.index) return;
				onReorder(
					moveIndicator(
						indicators.map(({ id }) => id),
						source.index,
						destination.index,
					),
				);
			}}
		>
			<Droppable droppableId="scanner-indicators">
				{(droppable) => (
					<div ref={droppable.innerRef} {...droppable.droppableProps}>
						{indicators.map((indicator, index) => (
							<Draggable
								draggableId={String(indicator.id)}
								index={index}
								isDragDisabled={disabled}
								key={indicator.id}
							>
								{(draggable) => (
									<Box
										mb="xs"
										ref={draggable.innerRef}
										{...draggable.draggableProps}
									>
										<ScannerIndicatorRow
											disabled={disabled}
											dragHandleProps={draggable.dragHandleProps}
											indicator={indicator}
											onDelete={onDelete}
										/>
									</Box>
								)}
							</Draggable>
						))}
						{droppable.placeholder}
					</div>
				)}
			</Droppable>
		</DragDropContext>
	);
}
