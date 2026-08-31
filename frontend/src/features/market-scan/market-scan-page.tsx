import {
	Anchor,
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
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { ApiError, fetchMarketScan } from "../../api/client";
import { useBusinessRequestPermission } from "../../app/business-request-context";
import { getTelegramInitData } from "../../app/telegram";
import { AnalysisCriteriaFields } from "../analysis/analysis-criteria-fields";
import { defaultPeriodForUnit } from "../analysis/criteria";
import { useAnalysisErrorNotification } from "../analysis/use-analysis-error-notification";
import { useAnalysisWarningNotification } from "../analysis/use-analysis-warning-notification";
import {
	criterionSelections,
	defaultMarketScanCriteria,
	type MarketScanCriteria,
	type MarketScanDraft,
	marketCapEvaluation,
	marketScanCriteriaConstraints,
	validateMarketScanCriteria,
	volatilityEvaluation,
} from "./criteria";
import {
	binanceSpotUrl,
	filterMarketScanItems,
	formatMarketCapUsd,
	formatRangePercent,
	hasRequiredMarketScanEvaluations,
	marketCapUnavailableReason,
} from "./results";

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
			const result = await fetchMarketScan(
				criterionSelections(committedCriteria),
				{
					initData: getTelegramInitData(),
				},
			);
			if (
				!hasRequiredMarketScanEvaluations(
					result.items,
					committedCriteria.minimumMarketCapMillions > 0,
				)
			) {
				throw new ApiError("unexpected_error");
			}
			return result;
		},
		queryKey: committedCriteria
			? marketScanQueryKey(committedCriteria)
			: (["market-scan", "uncommitted"] as const),
		retry: false,
		gcTime: Number.POSITIVE_INFINITY,
		staleTime: Number.POSITIVE_INFINITY,
	});
	useAnalysisErrorNotification(query.error, "Market Scan failed");
	useAnalysisWarningNotification(query.data?.warnings, "Market Scan warning");

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
	const marketCapEnabled =
		(committedCriteria?.minimumMarketCapMillions ?? 0) > 0;
	const filteredItems = query.data
		? filterMarketScanItems(query.data.items, symbolFilter).flatMap((item) => {
				const evaluation = volatilityEvaluation(item.evaluations);
				const marketCap = marketCapEvaluation(item.evaluations);
				return evaluation && (!marketCapEnabled || marketCap)
					? [{ evaluation, item, marketCap }]
					: [];
			})
		: [];

	return (
		<Container maw={880} px={0} size="md">
			<Stack gap="md">
				<Stack gap={2}>
					<Title order={1} size="h2">
						Market Scan
					</Title>
					<Text c="dimmed" size="sm">
						Find instruments by their candle range over a selected analysis
						period.
					</Text>
				</Stack>

				<Paper component="form" onSubmit={handleSubmit} p="md" withBorder>
					<Stack gap="sm">
						<AnalysisCriteriaFields
							percentileInputProps={form.getInputProps("percentile")}
							percentileKey={form.key("percentile")}
							periodInputProps={form.getInputProps("period")}
							periodKey={form.key("period")}
							unit={form.values.unit}
							onUnitChange={(unit) => {
								form.setValues({ unit, period: defaultPeriodForUnit(unit) });
							}}
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
						<NumberInput
							decimalScale={2}
							key={form.key("minimumMarketCapMillions")}
							label="Minimum Market Cap (millions)"
							min={
								marketScanCriteriaConstraints.minimumMarketCapMillions.minimum
							}
							prefix="$"
							step={1}
							suffix="M"
							{...form.getInputProps("minimumMarketCapMillions")}
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
									<Table.ScrollContainer minWidth={540}>
										<Table highlightOnHover verticalSpacing="xs">
											<Table.Thead>
												<Table.Tr>
													<Table.Th>Symbol</Table.Th>
													<Table.Th>Range Percent</Table.Th>
													<Table.Th>Candle Count</Table.Th>
													{marketCapEnabled ? (
														<Table.Th>Market Cap USD</Table.Th>
													) : null}
													<Table.Th ta="right">Binance</Table.Th>
												</Table.Tr>
											</Table.Thead>
											<Table.Tbody>
												{filteredItems.map(
													({ evaluation, item, marketCap }) => (
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
																{formatRangePercent(evaluation.rangePercent)}
															</Table.Td>
															<Table.Td>{evaluation.candleCount}</Table.Td>
															{marketCapEnabled && marketCap ? (
																<Table.Td>
																	{formatMarketCapUsd(marketCap.marketCapUsd)}
																</Table.Td>
															) : null}
															<Table.Td ta="right">
																{binanceSpotUrl(item.symbol) ? (
																	<Anchor
																		aria-label={`Open ${item.symbol} on Binance Spot in a new tab`}
																		href={binanceSpotUrl(item.symbol)}
																		onClick={(event) => event.stopPropagation()}
																		onKeyDown={(event) =>
																			event.stopPropagation()
																		}
																		rel="noopener noreferrer"
																		target="_blank"
																	>
																		Open ↗
																	</Anchor>
																) : (
																	<Text c="dimmed">—</Text>
																)}
															</Table.Td>
														</Table.Tr>
													),
												)}
											</Table.Tbody>
										</Table>
									</Table.ScrollContainer>
								</Paper>
							)}

							{query.data.unresolved.length > 0 ? (
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
													{query.data.unresolved.map((item) => (
														<Table.Tr key={`${item.symbol}-${item.code}`}>
															<Table.Td>{item.symbol}</Table.Td>
															<Table.Td>
																{marketCapUnavailableReason(item.code)}
															</Table.Td>
														</Table.Tr>
													))}
												</Table.Tbody>
											</Table>
										</Table.ScrollContainer>
									</Paper>
								</Stack>
							) : null}
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
		criteria.period,
		criteria.unit,
		criteria.percentile,
		criteria.minimumRangePercent,
		criteria.minimumMarketCapMillions,
	] as const;
}

function criteriaFromValidDraft(
	values: MarketScanDraft,
): MarketScanCriteria | undefined {
	if (
		typeof values.period !== "number" ||
		typeof values.percentile !== "number" ||
		typeof values.minimumMarketCapMillions !== "number" ||
		typeof values.minimumRangePercent !== "number"
	) {
		return undefined;
	}

	return {
		minimumMarketCapMillions: values.minimumMarketCapMillions,
		minimumRangePercent: values.minimumRangePercent,
		percentile: values.percentile,
		period: values.period,
		unit: values.unit,
	};
}
