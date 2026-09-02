import {
	Anchor,
	Box,
	Group,
	LoadingOverlay,
	Paper,
	Stack,
	Table,
	Text,
	TextInput,
	Title,
	UnstyledButton,
} from "@mantine/core";
import { useState } from "react";
import type { MarketScanItem, MarketScanResult } from "@/api/client";
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

export function MarketScanResults({
	criteria,
	isRefreshing,
	onSelectInstrument,
	onSortChange,
	result,
	sort,
}: MarketScanResultsProps) {
	const [symbolFilter, setSymbolFilter] = useState("");
	const marketCapEnabled = criteria.minimumMarketCapMillions > 0;
	const items = marketScanRows(
		sortMarketScanItems(
			filterMarketScanItems(result.items, symbolFilter),
			sort,
		),
		marketCapEnabled,
	);

	return (
		<Box pos="relative">
			<LoadingOverlay
				loaderProps={{ "aria-label": "Refreshing Market Scan" }}
				overlayProps={{ blur: 1 }}
				visible={isRefreshing}
				zIndex={10}
			/>
			<Stack gap="sm">
				<ScanSummary result={result} />
				<TextInput
					aria-label="Filter current Scan Result by symbol"
					description="Filters only the current Scan Result"
					label="Symbol filter"
					onChange={(event) => setSymbolFilter(event.currentTarget.value)}
					placeholder="e.g. BTC"
					value={symbolFilter}
				/>

				{result.items.length === 0 ? (
					<EmptyResult />
				) : items.length === 0 ? (
					<EmptyFilterResult />
				) : (
					<MarketScanTable
						criteria={criteria}
						items={items}
						marketCapEnabled={marketCapEnabled}
						onSelectInstrument={onSelectInstrument}
						onSortChange={onSortChange}
						sort={sort}
					/>
				)}

				{marketCapEnabled && result.unresolved.length > 0 ? (
					<UnresolvedMarketCapTable unresolved={result.unresolved} />
				) : null}
			</Stack>
		</Box>
	);
}

function ScanSummary({ result }: { result: MarketScanResult }) {
	return (
		<Paper p="sm" withBorder>
			<Group gap="lg">
				<Text size="sm">
					Matched <Text component="strong">{result.matched_count}</Text>
				</Text>
				<Text size="sm">
					Analyzed <Text component="strong">{result.analyzed_count}</Text>
				</Text>
				<Text size="sm">
					Insufficient data{" "}
					<Text component="strong">{result.insufficient_data_count}</Text>
				</Text>
			</Group>
		</Paper>
	);
}

function EmptyResult() {
	return (
		<Paper p="xl" ta="center" withBorder>
			<Text fw={600}>No instruments matched these criteria.</Text>
			<Text c="dimmed" mt={4} size="sm">
				Adjust the criteria and run another Market Scan.
			</Text>
		</Paper>
	);
}

function EmptyFilterResult() {
	return (
		<Paper p="xl" ta="center" withBorder>
			<Text fw={600}>No instruments match this symbol filter.</Text>
			<Text c="dimmed" mt={4} size="sm">
				Clear or change the filter to see this Scan Result.
			</Text>
		</Paper>
	);
}

interface ResultRow {
	daily: NonNullable<ReturnType<typeof volatilityEvaluation>>;
	hourly: NonNullable<ReturnType<typeof volatilityEvaluation>>;
	item: MarketScanItem;
	marketCap: ReturnType<typeof marketCapEvaluation>;
}

function marketScanRows(
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

interface MarketScanTableProps {
	criteria: MarketScanCriteria;
	items: ResultRow[];
	marketCapEnabled: boolean;
	onSelectInstrument(
		symbol: string,
		criteria: MarketScanCriteria,
	): Promise<void>;
	onSortChange(sort: MarketScanSort): Promise<void>;
	sort: MarketScanSort;
}

function MarketScanTable({
	criteria,
	items,
	marketCapEnabled,
	onSelectInstrument,
	onSortChange,
	sort,
}: MarketScanTableProps) {
	return (
		<Paper p="xs" withBorder>
			<Table.ScrollContainer minWidth={760}>
				<Table highlightOnHover verticalSpacing="xs">
					<Table.Thead>
						<Table.Tr>
							<Table.Th>Symbol</Table.Th>
							<SortableMarketScanHeader
								column="daily_volatility"
								label="Daily Range"
								onSortChange={onSortChange}
								sort={sort}
							/>
							<Table.Th>Daily Candle Count</Table.Th>
							<SortableMarketScanHeader
								column="hourly_volatility"
								label="Hourly Range"
								onSortChange={onSortChange}
								sort={sort}
							/>
							<Table.Th>Hourly Candle Count</Table.Th>
							{marketCapEnabled ? (
								<SortableMarketScanHeader
									column="market_cap"
									label="Market Cap USD"
									onSortChange={onSortChange}
									sort={sort}
								/>
							) : null}
							<Table.Th ta="right">Binance</Table.Th>
						</Table.Tr>
					</Table.Thead>
					<Table.Tbody>
						{items.map(({ daily, hourly, item, marketCap }) => (
							<Table.Tr
								key={item.symbol}
								onClick={() => void onSelectInstrument(item.symbol, criteria)}
								onKeyDown={(event) => {
									if (event.key === "Enter")
										void onSelectInstrument(item.symbol, criteria);
								}}
								role="link"
								style={{ cursor: "pointer" }}
								tabIndex={0}
							>
								<Table.Td>{item.symbol}</Table.Td>
								<Table.Td>{formatRangePercent(daily.rangePercent)}</Table.Td>
								<Table.Td>{daily.candleCount}</Table.Td>
								<Table.Td>{formatRangePercent(hourly.rangePercent)}</Table.Td>
								<Table.Td>{hourly.candleCount}</Table.Td>
								{marketCapEnabled && marketCap ? (
									<Table.Td>
										{formatMarketCapUsd(marketCap.marketCapUsd)}
									</Table.Td>
								) : null}
								<Table.Td ta="right">
									<BinanceLink symbol={item.symbol} />
								</Table.Td>
							</Table.Tr>
						))}
					</Table.Tbody>
				</Table>
			</Table.ScrollContainer>
		</Paper>
	);
}

function BinanceLink({ symbol }: { symbol: string }) {
	const url = binanceSpotUrl(symbol);
	return url ? (
		<Anchor
			aria-label={`Open ${symbol} on Binance Spot in a new tab`}
			href={url}
			onClick={(event) => event.stopPropagation()}
			onKeyDown={(event) => event.stopPropagation()}
			rel="noopener noreferrer"
			target="_blank"
		>
			Open ↗
		</Anchor>
	) : (
		<Text c="dimmed">—</Text>
	);
}

function UnresolvedMarketCapTable({
	unresolved,
}: {
	unresolved: MarketScanResult["unresolved"];
}) {
	return (
		<Stack gap="xs">
			<Title order={2} size="h4">
				Instruments with unavailable Market Cap
			</Title>
			<Paper p="xs" withBorder>
				<Table.ScrollContainer minWidth={420}>
					<Table highlightOnHover verticalSpacing="xs">
						<Table.Thead>
							<Table.Tr>
								<Table.Th>Symbol</Table.Th>
								<Table.Th>Reason</Table.Th>
							</Table.Tr>
						</Table.Thead>
						<Table.Tbody>
							{unresolved.map((item) => (
								<Table.Tr key={`${item.symbol}-${item.code}`}>
									<Table.Td>{item.symbol}</Table.Td>
									<Table.Td>{marketCapUnavailableReason(item.code)}</Table.Td>
								</Table.Tr>
							))}
						</Table.Tbody>
					</Table>
				</Table.ScrollContainer>
			</Paper>
		</Stack>
	);
}

function SortableMarketScanHeader({
	column,
	label,
	onSortChange,
	sort,
}: {
	column: MarketScanSortColumn;
	label: string;
	onSortChange(sort: MarketScanSort): Promise<void>;
	sort: MarketScanSort;
}) {
	const active = sort.column === column;
	const ariaSort = !active
		? "none"
		: sort.direction === "desc"
			? "descending"
			: "ascending";
	return (
		<Table.Th aria-sort={ariaSort}>
			<UnstyledButton
				aria-label={`Sort by ${label}${active ? `, currently ${sort.direction}ending` : ""}`}
				onClick={() => void onSortChange(nextMarketScanSort(sort, column))}
			>
				{label}
				{active ? (sort.direction === "desc" ? " ↓" : " ↑") : null}
			</UnstyledButton>
		</Table.Th>
	);
}
