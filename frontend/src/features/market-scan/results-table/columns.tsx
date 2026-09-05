import type { ReactNode } from "react";
import type { PriceHistoryWindow, UnresolvedInstrument } from "@/api/client";
import type { DataTableColumn } from "@/components/data-table";
import { PriceHistoryChart } from "@/features/market-scan/price-history-chart";
import { BinanceLink } from "@/features/market-scan/results-table/binance-link";
import { marketScanColumnKeys } from "@/features/market-scan/results-table/keys";
import {
	type MarketScanRow,
	marketCapUnavailableReason,
} from "@/features/market-scan/results-table/utils";
import { formatMarketCapUsd } from "@/utils/market-cap";
import { formatRangePercent } from "@/utils/range-percent";

interface MarketScanColumn {
	key: (typeof marketScanColumnKeys)[keyof typeof marketScanColumnKeys];
	header: string;
	cell(row: MarketScanRow, window: PriceHistoryWindow): ReactNode;
	sortable?: boolean;
	textAlign?: DataTableColumn<MarketScanRow>["textAlign"];
}

// This is the complete, ordered table definition. Criteria do not configure it.
export const marketScanColumns = [
	{
		key: marketScanColumnKeys.symbol,
		header: "Symbol",
		cell: (row) => row.symbol,
	},
	{
		key: marketScanColumnKeys.dailyRange,
		header: "Daily Range",
		cell: (row) =>
			row.dailyRangePercent === null
				? "—"
				: formatRangePercent(row.dailyRangePercent),
		sortable: true,
	},
	{
		key: marketScanColumnKeys.hourlyRange,
		header: "Hourly Range",
		cell: (row) =>
			row.hourlyRangePercent === null
				? "—"
				: formatRangePercent(row.hourlyRangePercent),
		sortable: true,
	},
	{
		key: marketScanColumnKeys.hourlyCandleCount,
		header: "Hourly Candle Count",
		cell: (row) => row.hourlyCandleCount ?? "—",
	},
	{
		key: marketScanColumnKeys.dailyCandleCount,
		header: "Daily Candle Count",
		cell: (row) => row.dailyCandleCount ?? "—",
	},
	{
		key: marketScanColumnKeys.marketCap,
		header: "Market Cap USD",
		cell: (row) =>
			row.marketCapUsd === null ? "—" : formatMarketCapUsd(row.marketCapUsd),
		sortable: true,
	},
	{
		key: marketScanColumnKeys.priceHistory,
		header: "7d change",
		cell: (row, window) => (
			<PriceHistoryChart
				prices={row.priceHistory}
				symbol={row.symbol}
				window={window}
			/>
		),
	},
	{
		key: marketScanColumnKeys.sevenDayChangePercent,
		header: "7d change percent",
		cell: (row) => {
			const change = row.sevenDayChangePercent;
			if (change === null) return "—";

			const color =
				change > 0
					? "var(--mantine-color-green-6)"
					: change < 0
						? "var(--mantine-color-red-6)"
						: undefined;
			return <span style={{ color }}>{formatRangePercent(change)}</span>;
		},
		sortable: true,
	},
	{
		key: marketScanColumnKeys.binance,
		header: "Binance",
		cell: (row) => <BinanceLink symbol={row.symbol} />,
		textAlign: "center",
	},
] as const satisfies readonly MarketScanColumn[];

export type MarketScanSortColumn = Extract<
	(typeof marketScanColumns)[number],
	{ sortable: true }
>["key"];

export const unresolvedInstrumentColumns: readonly DataTableColumn<UnresolvedInstrument>[] =
	[
		{
			key: marketScanColumnKeys.symbol,
			header: "Symbol",
			cell: (item) => item.symbol,
		},
		{
			key: marketScanColumnKeys.reason,
			header: "Reason",
			cell: (item) => marketCapUnavailableReason(item.code),
		},
	];
