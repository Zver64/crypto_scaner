import type { ReactNode } from "react";
import type {
	PriceHistoryWindow,
	TableCell,
	TableColumn,
	TableColumnKind,
	TableRow,
} from "@/api/generated/models";
import { ExternalLink } from "@/components/external-link";
import { PercentChange } from "@/components/percent-change";
import { FavoriteToggle } from "@/features/favorites/favorite-toggle";
import { PriceHistoryChart } from "@/features/market-scan/price-history-chart";
import { oscillatorColor } from "@/features/market-scan/results-table/utils";
import { formatMarketCapUsd } from "@/utils/market-cap";
import { formatNumber } from "@/utils/number-format";
import { formatRangePercent } from "@/utils/range-percent";

interface CellContext {
	cell: TableCell | undefined;
	column: TableColumn;
	row: TableRow;
	window: PriceHistoryWindow | undefined;
}

export type CellRenderer = (context: CellContext) => ReactNode;

// Every kind the backend can send needs a renderer here; columns themselves
// come from the backend table.
export const cellRenderers: Record<TableColumnKind, CellRenderer> = {
	text: ({ row }) => row.symbol,
	usd_compact: ({ cell }) =>
		cell?.value == null ? "—" : formatMarketCapUsd(cell.value),
	range_percent: ({ cell }) =>
		cell?.value == null ? "—" : formatRangePercent(cell.value),
	oscillator: ({ cell }) => <OscillatorValue value={cell?.value ?? null} />,
	percent_change: ({ cell }) => <PercentChange value={cell?.value ?? null} />,
	sparkline: ({ cell, row, window }) =>
		window && cell?.series ? (
			<PriceHistoryChart
				prices={cell.series}
				symbol={row.symbol}
				window={window}
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

function OscillatorValue({ value }: { value: number | null }) {
	if (value === null) {
		return "—";
	}
	return (
		<span style={{ color: oscillatorColor(value) }}>{value.toFixed(1)}</span>
	);
}
