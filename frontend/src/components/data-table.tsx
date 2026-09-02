import { Paper, Table } from "@mantine/core";
import type { ReactNode } from "react";

export interface DataTableColumn<Row> {
	ariaSort?: "ascending" | "descending" | "none";
	cell(row: Row): ReactNode;
	header: ReactNode;
	key: string;
	textAlign?: "center" | "left" | "right";
}

interface DataTableProps<Row> {
	columns: readonly DataTableColumn<Row>[];
	getRowKey(row: Row): string;
	minWidth: number;
	onRowClick?(row: Row): void;
	rows: readonly Row[];
}

export function DataTable<Row>({
	columns,
	getRowKey,
	minWidth,
	onRowClick,
	rows,
}: DataTableProps<Row>) {
	return (
		<Paper p="xs" withBorder>
			<Table.ScrollContainer minWidth={minWidth}>
				<Table highlightOnHover verticalSpacing="xs">
					<Table.Thead>
						<Table.Tr>
							{columns.map((column) => (
								<Table.Th
									aria-sort={column.ariaSort}
									key={column.key}
									ta={column.textAlign}
								>
									{column.header}
								</Table.Th>
							))}
						</Table.Tr>
					</Table.Thead>
					<Table.Tbody>
						{rows.map((row) => (
							<Table.Tr
								key={getRowKey(row)}
								onClick={onRowClick ? () => onRowClick(row) : undefined}
								onKeyDown={
									onRowClick
										? (event) => {
												if (event.key === "Enter") onRowClick(row);
											}
										: undefined
								}
								role={onRowClick ? "link" : undefined}
								style={onRowClick ? { cursor: "pointer" } : undefined}
								tabIndex={onRowClick ? 0 : undefined}
							>
								{columns.map((column) => (
									<Table.Td key={column.key} ta={column.textAlign}>
										{column.cell(row)}
									</Table.Td>
								))}
							</Table.Tr>
						))}
					</Table.Tbody>
				</Table>
			</Table.ScrollContainer>
		</Paper>
	);
}
