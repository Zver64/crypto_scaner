import { Text } from "@mantine/core";
import type { ReactNode } from "react";

import {
	MarketUniverse,
	type TableCell,
	type TableColumn,
	type TableColumnKind,
	type TableRow,
} from "@/api/generated/models";
import { ExternalLink } from "@/components/external-link";
import { PercentChange } from "@/components/percent-change";
import { FavoriteToggle } from "@/features/favorites/favorite-toggle";
import { PriceSparkline } from "@/features/market-scan/price-sparkline";
import { OscillatorValue } from "@/features/market-scan/results-table/oscillator-value";
import { formatMarketCapUsd } from "@/utils/market-cap";
import { formatNumber } from "@/utils/number-format";
import { formatRangePercent } from "@/utils/range-percent";

interface CellContext {
	cell: TableCell | undefined;
	column: TableColumn;
	highlightCoinM: boolean;
	row: TableRow;
}

export type CellRenderer = (context: CellContext) => ReactNode;

// Every kind the backend can send needs a renderer here; columns themselves
// come from the backend table.
export const cellRenderers: Record<TableColumnKind, CellRenderer> = {
	text: ({ highlightCoinM, row }) =>
		highlightCoinM && row.markets.includes(MarketUniverse.coinm_perpetual) ? (
			<Text c="teal" inherit span>
				{row.symbol}
			</Text>
		) : (
			row.symbol
		),
	usd_compact: ({ cell }) =>
		cell?.value == null ? "—" : formatMarketCapUsd(cell.value),
	range_percent: ({ cell }) =>
		cell?.value == null ? "—" : formatRangePercent(cell.value),
	oscillator: ({ cell }) => <OscillatorValue value={cell?.value ?? null} />,
	percent_change: ({ cell }) => <PercentChange value={cell?.value ?? null} />,
	sparkline: ({ cell, column, row }) =>
		column.window && cell?.series ? (
			<PriceSparkline
				prices={cell.series}
				symbol={row.symbol}
				title={column.title}
				window={column.window}
			/>
		) : (
			"—"
		),
	link: ({ cell, column, row }) =>
		cell?.url ? (
			<ExternalLink
				ariaLabel={`Open ${row.symbol} on ${column.title}`}
				href={cell.url}
			/>
		) : (
			"—"
		),
	favorite: ({ row }) => <FavoriteToggle symbol={row.symbol} />,
	count: ({ cell }) => cell?.value ?? "—",
	number: ({ cell }) => (cell?.value == null ? "—" : formatNumber(cell.value)),
};
