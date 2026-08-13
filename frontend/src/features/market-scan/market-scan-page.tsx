import {
	Box,
	Button,
	Center,
	Container,
	Group,
	Loader,
	LoadingOverlay,
	NumberInput,
	Paper,
	Stack,
	Table,
	Text,
	TextInput,
	Title,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { ApiError, fetchMarketScan } from "../../api/client";
import { useBusinessRequestPermission } from "../../app/business-request-context";
import { getTelegramInitData } from "../../app/telegram";
import {
	defaultMarketScanCriteria,
	type MarketScanCriteria,
	type MarketScanDraft,
	marketScanCriteriaConstraints,
	validateMarketScanCriteria,
} from "./criteria";
import { filterMarketScanItems, formatRangePercent } from "./results";

export function marketScanQueryKey(criteria: MarketScanCriteria) {
	return ["market-scan", ...marketScanCriteriaIdentity(criteria)] as const;
}

interface MarketScanPageProps {
	committedCriteria: MarketScanCriteria | undefined;
	onCommit(criteria: MarketScanCriteria): Promise<void>;
	onSelectInstrument(
		symbol: string,
		criteria: MarketScanCriteria,
	): Promise<void>;
}

export function MarketScanPage({
	committedCriteria,
	onCommit,
	onSelectInstrument,
}: MarketScanPageProps) {
	const permission = useBusinessRequestPermission();
	const [symbolFilter, setSymbolFilter] = useState("");
	const form = useForm<MarketScanDraft>({
		initialValues: committedCriteria ?? defaultMarketScanCriteria,
		mode: "controlled",
		validate: validateMarketScanCriteria,
		validateInputOnChange: true,
	});
	const query = useQuery({
		enabled: committedCriteria !== undefined && permission.allowed,
		queryFn: async () => {
			if (!committedCriteria) {
				throw new ApiError("unexpected_error");
			}
			return fetchMarketScan(committedCriteria, {
				initData: getTelegramInitData(),
			});
		},
		queryKey: committedCriteria
			? marketScanQueryKey(committedCriteria)
			: (["market-scan", "uncommitted"] as const),
		retry: false,
		staleTime: Number.POSITIVE_INFINITY,
	});

	useEffect(() => {
		if (!query.error) {
			return;
		}

		notifications.show({
			autoClose: 5000,
			color: "red",
			message:
				query.error instanceof ApiError
					? query.error.message
					: "An unexpected error occurred. Please try again.",
			title: "Market Scan failed",
		});
	}, [query.error]);

	const handleSubmit = form.onSubmit(async (values) => {
		const criteria = criteriaFromValidDraft(values);
		if (!criteria) {
			return;
		}

		if (committedCriteria && criteriaAreEqual(criteria, committedCriteria)) {
			await query.refetch();
			return;
		}

		await onCommit(criteria);
	});
	const submissionDisabled =
		!form.isValid() || !permission.allowed || query.isFetching;
	const filteredItems = query.data
		? filterMarketScanItems(query.data.items, symbolFilter)
		: [];

	return (
		<Container maw={880} px={0} size="md">
			<Stack gap="md">
				<Stack gap={2}>
					<Title order={1} size="h2">
						Market Scan
					</Title>
					<Text c="dimmed" size="sm">
						Find instruments by their daily range over a selected analysis
						period.
					</Text>
				</Stack>

				<Paper component="form" onSubmit={handleSubmit} p="md" withBorder>
					<Stack gap="sm">
						<NumberInput
							allowDecimal={false}
							key={form.key("periodDays")}
							label="Analysis Period"
							max={marketScanCriteriaConstraints.periodDays.maximum}
							min={marketScanCriteriaConstraints.periodDays.minimum}
							suffix=" days"
							{...form.getInputProps("periodDays")}
						/>
						<NumberInput
							allowDecimal={false}
							key={form.key("percentile")}
							label="Range Percentile"
							max={marketScanCriteriaConstraints.percentile.maximum}
							min={marketScanCriteriaConstraints.percentile.minimum}
							{...form.getInputProps("percentile")}
						/>
						<NumberInput
							decimalScale={10}
							key={form.key("minimumRangePercent")}
							label="Minimum Range"
							min={marketScanCriteriaConstraints.minimumRangePercent.minimum}
							step={0.1}
							suffix="%"
							{...form.getInputProps("minimumRangePercent")}
						/>
						<Button
							disabled={submissionDisabled}
							loading={query.isFetching}
							type="submit"
						>
							Run Market Scan
						</Button>
					</Stack>
				</Paper>

				{committedCriteria && query.isFetching && !query.data ? (
					<Center mih={180}>
						<Loader aria-label="Loading Market Scan" />
					</Center>
				) : null}

				{query.data && committedCriteria ? (
					<Box pos="relative">
						<LoadingOverlay
							loaderProps={{ "aria-label": "Refreshing Market Scan" }}
							overlayProps={{ blur: 1 }}
							visible={query.isFetching}
							zIndex={10}
						/>
						<Stack gap="sm">
							<Paper p="sm" withBorder>
								<Group gap="lg">
									<Text size="sm">
										Matched{" "}
										<Text component="strong">{query.data.matched_count}</Text>
									</Text>
									<Text size="sm">
										Analyzed{" "}
										<Text component="strong">{query.data.analyzed_count}</Text>
									</Text>
									<Text size="sm">
										Insufficient data{" "}
										<Text component="strong">
											{query.data.insufficient_data_count}
										</Text>
									</Text>
								</Group>
							</Paper>

							<TextInput
								aria-label="Filter current Scan Result by symbol"
								description="Filters only the current Scan Result"
								label="Symbol filter"
								onChange={(event) => setSymbolFilter(event.currentTarget.value)}
								placeholder="e.g. BTC"
								value={symbolFilter}
							/>

							{query.data.items.length === 0 ? (
								<Paper p="xl" ta="center" withBorder>
									<Text fw={600}>No instruments matched these criteria.</Text>
									<Text c="dimmed" mt={4} size="sm">
										Adjust the criteria and run another Market Scan.
									</Text>
								</Paper>
							) : filteredItems.length === 0 ? (
								<Paper p="xl" ta="center" withBorder>
									<Text fw={600}>No instruments match this symbol filter.</Text>
									<Text c="dimmed" mt={4} size="sm">
										Clear or change the filter to see this Scan Result.
									</Text>
								</Paper>
							) : (
								<Paper p="xs" withBorder>
									<Table.ScrollContainer minWidth={420}>
										<Table highlightOnHover verticalSpacing="xs">
											<Table.Thead>
												<Table.Tr>
													<Table.Th>Symbol</Table.Th>
													<Table.Th>Range Percent</Table.Th>
													<Table.Th>Candle Count</Table.Th>
												</Table.Tr>
											</Table.Thead>
											<Table.Tbody>
												{filteredItems.map((item) => (
													<Table.Tr
														key={item.symbol}
														onClick={() =>
															void onSelectInstrument(
																item.symbol,
																committedCriteria,
															)
														}
														onKeyDown={(event) => {
															if (event.key === "Enter") {
																void onSelectInstrument(
																	item.symbol,
																	committedCriteria,
																);
															}
														}}
														role="link"
														style={{ cursor: "pointer" }}
														tabIndex={0}
													>
														<Table.Td>{item.symbol}</Table.Td>
														<Table.Td>
															{formatRangePercent(item.range_percent)}
														</Table.Td>
														<Table.Td>{item.candle_count}</Table.Td>
													</Table.Tr>
												))}
											</Table.Tbody>
										</Table>
									</Table.ScrollContainer>
								</Paper>
							)}
						</Stack>
					</Box>
				) : null}
			</Stack>
		</Container>
	);
}

function criteriaAreEqual(
	left: MarketScanCriteria,
	right: MarketScanCriteria,
): boolean {
	const leftIdentity = marketScanCriteriaIdentity(left);
	const rightIdentity = marketScanCriteriaIdentity(right);
	return leftIdentity.every((value, index) => value === rightIdentity[index]);
}

function marketScanCriteriaIdentity(criteria: MarketScanCriteria) {
	return [
		criteria.periodDays,
		criteria.percentile,
		criteria.minimumRangePercent,
	] as const;
}

function criteriaFromValidDraft(
	values: MarketScanDraft,
): MarketScanCriteria | undefined {
	if (
		typeof values.periodDays !== "number" ||
		typeof values.percentile !== "number" ||
		typeof values.minimumRangePercent !== "number"
	) {
		return undefined;
	}

	return {
		minimumRangePercent: values.minimumRangePercent,
		percentile: values.percentile,
		periodDays: values.periodDays,
	};
}
