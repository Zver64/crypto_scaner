import {
	Anchor,
	Group,
	Paper,
	Stack,
	Text,
	TextInput,
	Title,
	UnstyledButton,
	useMatches,
} from "@mantine/core";
import { useState } from "react";
import type { MarketScanItem, MarketScanResult } from "@/api/client";
import { openTelegramExternalLink } from "@/app/telegram";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import { marketCapEvaluation, volatilityEvaluation } from "./criteria";
import {
	dailyVolatilityKey,
	hourlyVolatilityKey,
	type MarketScanCriteria,
} from "./pipeline";
import {
	binanceSpotUrl,
	filterMarketScanItems,
	formatMarketCapUsd,
	formatRangePercent,
	marketCapUnavailableReason,
	sortMarketScanItems,
} from "./results";
import {
	type MarketScanSort,
	type MarketScanSortColumn,
	nextMarketScanSort,
} from "./sort";

interface MarketScanResultsProps {
	criteria: MarketScanCriteria;
	isRefreshing: boolean;
	onSelectInstrument(
		symbol: string,
		criteria: MarketScanCriteria,
	): Promise<void>;
	onSortChange(sort: MarketScanSort): Promise<void>;
	result: MarketScanResult;
	sort: MarketScanSort;
}

interface ResultRow {
	daily: NonNullable<ReturnType<typeof volatilityEvaluation>>;
	hourly: NonNullable<ReturnType<typeof volatilityEvaluation>>;
	item: MarketScanItem;
	marketCap: ReturnType<typeof marketCapEvaluation>;
}

export function MarketScanResults({
	criteria,
	isRefreshing,
	onSelectInstrument,
	onSortChange,
	result,
	sort,
}: MarketScanResultsProps) {
	const contentSpacing = useMatches({ base: "xs", sm: "sm" });
	const iconSize = useMatches({ base: 16, sm: 18 });
	const textSize = useMatches({ base: "xs", sm: "sm" });
	const [symbolFilter, setSymbolFilter] = useState("");
	const marketCapEnabled = criteria.minimumMarketCapMillions > 0;
	const rows = toResultRows(
		sortMarketScanItems(
			filterMarketScanItems(result.items, symbolFilter),
			sort,
		),
		marketCapEnabled,
	);
	const marketCapColumn: DataTableColumn<ResultRow> | undefined =
		marketCapEnabled
			? {
					ariaSort: sort.column === "market_cap" ? sortDirection(sort) : "none",
					cell: ({ marketCap }) =>
						marketCap ? formatMarketCapUsd(marketCap.marketCapUsd) : "—",
					header: sortableHeader(
						"market_cap",
						"Market Cap USD",
						sort,
						onSortChange,
					),
					key: "market-cap",
				}
			: undefined;
	const columns: DataTableColumn<ResultRow>[] = [
		{
			cell: ({ item }) => item.symbol,
			header: "Symbol",
			key: "symbol",
		},
		sortableColumn(
			"daily_volatility",
			"Daily Range",
			sort,
			onSortChange,
			({ daily }) => formatRangePercent(daily.rangePercent),
		),
		sortableColumn(
			"hourly_volatility",
			"Hourly Range",
			sort,
			onSortChange,
			({ hourly }) => formatRangePercent(hourly.rangePercent),
		),
		{
			cell: ({ hourly }) => hourly.candleCount,
			header: "Hourly Candle Count",
			key: "hourly-candle-count",
		},
		{
			cell: ({ daily }) => daily.candleCount,
			header: "Daily Candle Count",
			key: "daily-candle-count",
		},
		...(marketCapColumn ? [marketCapColumn] : []),
		{
			cell: ({ item }) => {
				const url = binanceSpotUrl(item.symbol);
				return url ? (
					<Anchor
						aria-label={`Open ${item.symbol} on Binance Spot`}
						href={url}
						onClick={(event) => {
							event.stopPropagation();
							if (openTelegramExternalLink(url)) {
								event.preventDefault();
							}
						}}
						onKeyDown={(event) => event.stopPropagation()}
						rel="noopener noreferrer"
						style={{
							display: "inline-flex",
							lineHeight: 0,
							verticalAlign: "middle",
						}}
						target="_blank"
					>
						<svg
							aria-hidden="true"
							data-icon="external-link"
							fill="none"
							height={iconSize}
							stroke="currentColor"
							strokeLinecap="round"
							strokeLinejoin="round"
							strokeWidth="2"
							viewBox="0 0 24 24"
							width={iconSize}
						>
							<path d="m6 18 12-12" />
							<path d="M9 6h9v9" />
						</svg>
					</Anchor>
				) : (
					<Text c="dimmed">—</Text>
				);
			},
			header: "Binance",
			key: "binance",
			textAlign: "center",
		},
	];
	const unresolvedColumns: DataTableColumn<
		MarketScanResult["unresolved"][number]
	>[] = [
		{ cell: (item) => item.symbol, header: "Symbol", key: "symbol" },
		{
			cell: (item) => marketCapUnavailableReason(item.code),
			header: "Reason",
			key: "reason",
		},
	];

	return (
		<RefreshingOverlay label="Refreshing Market Scan" visible={isRefreshing}>
			<Stack gap={contentSpacing}>
				<Paper p={contentSpacing} withBorder>
					<Group gap="lg">
						<Text size={textSize}>
							Matched <Text component="strong">{result.matched_count}</Text>
						</Text>
						<Text size={textSize}>
							Analyzed <Text component="strong">{result.analyzed_count}</Text>
						</Text>
						<Text size={textSize}>
							Insufficient data{" "}
							<Text component="strong">{result.insufficient_data_count}</Text>
						</Text>
					</Group>
				</Paper>
				<TextInput
					aria-label="Filter current Scan Result by symbol"
					description="Filters only the current Scan Result"
					label="Symbol filter"
					size={textSize}
					onChange={(event) => setSymbolFilter(event.currentTarget.value)}
					placeholder="e.g. BTC"
					value={symbolFilter}
				/>
				{result.items.length === 0 ? (
					<Paper p="xl" ta="center" withBorder>
						<Text fw={600}>No instruments matched these criteria.</Text>
						<Text c="dimmed" mt={4} size="sm">
							Adjust the criteria and run another Market Scan.
						</Text>
					</Paper>
				) : rows.length === 0 ? (
					<Paper p="xl" ta="center" withBorder>
						<Text fw={600}>No instruments match this symbol filter.</Text>
						<Text c="dimmed" mt={4} size="sm">
							Clear or change the filter to see this Scan Result.
						</Text>
					</Paper>
				) : (
					<DataTable
						columns={columns}
						getRowKey={({ item }) => item.symbol}
						minWidth={760}
						onRowClick={({ item }) =>
							void onSelectInstrument(item.symbol, criteria)
						}
						rows={rows}
					/>
				)}
				{marketCapEnabled && result.unresolved.length > 0 ? (
					<Stack gap="xs">
						<Title order={2} size={textSize === "xs" ? "h5" : "h4"}>
							Instruments with unavailable Market Cap
						</Title>
						<DataTable
							columns={unresolvedColumns}
							getRowKey={(item) => `${item.symbol}-${item.code}`}
							minWidth={420}
							rows={result.unresolved}
						/>
					</Stack>
				) : null}
			</Stack>
		</RefreshingOverlay>
	);
}

function sortableColumn(
	column: MarketScanSortColumn,
	label: string,
	sort: MarketScanSort,
	onSortChange: (sort: MarketScanSort) => Promise<void>,
	cell: DataTableColumn<ResultRow>["cell"],
): DataTableColumn<ResultRow> {
	return {
		ariaSort: sort.column === column ? sortDirection(sort) : "none",
		cell,
		header: sortableHeader(column, label, sort, onSortChange),
		key: column,
	};
}

function sortableHeader(
	column: MarketScanSortColumn,
	label: string,
	sort: MarketScanSort,
	onSortChange: (sort: MarketScanSort) => Promise<void>,
) {
	const active = sort.column === column;
	return (
		<UnstyledButton
			aria-label={`Sort by ${label}${active ? `, currently ${sort.direction}ending` : ""}`}
			onClick={() => void onSortChange(nextMarketScanSort(sort, column))}
		>
			{label}
			{active ? (sort.direction === "desc" ? " ↓" : " ↑") : null}
		</UnstyledButton>
	);
}

function sortDirection(sort: MarketScanSort): "ascending" | "descending" {
	return sort.direction === "desc" ? "descending" : "ascending";
}

function toResultRows(
	items: readonly MarketScanItem[],
	marketCapEnabled: boolean,
): ResultRow[] {
	return items.flatMap((item) => {
		const daily = volatilityEvaluation(item.evaluations, dailyVolatilityKey);
		const hourly = volatilityEvaluation(item.evaluations, hourlyVolatilityKey);
		const marketCap = marketCapEvaluation(item.evaluations);
		return daily && hourly && (!marketCapEnabled || marketCap)
			? [{ daily, hourly, item, marketCap }]
			: [];
	});
}
