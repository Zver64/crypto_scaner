import {
	Button,
	Center,
	Container,
	Group,
	Loader,
	Paper,
	SimpleGrid,
	Stack,
	Text,
	Title,
	useMatches,
} from "@mantine/core";
import {
	criterionKeys,
	evaluationMetricKeys,
} from "@/api/analysis-identifiers";
import { ApiError, type CriterionSelection } from "@/api/client";
import { useInstrumentAnalysisQuery } from "@/api/instrument-analysis";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { useTelegramBackButton } from "@/app/telegram";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import { useAnalysisErrorNotification } from "@/features/analysis/use-analysis-error-notification";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import { PriceHistoryChart } from "@/features/market-scan/price-history-chart";
import { formatMarketCapUsd, marketCapEvaluation } from "@/utils/market-cap";
import { formatRangePercent } from "@/utils/range-percent";

const rangeStatistics = [
	{
		key: criterionKeys.dailyVolatility,
		label: "Daily Range",
		stepLabel: "Daily Grid Step",
		coverageLabel: "Days available",
	},
	{
		key: criterionKeys.hourlyVolatility,
		label: "Hourly Range",
		stepLabel: "Hourly Grid Step",
		coverageLabel: "Hours available",
	},
];

interface InstrumentAnalysisScreenProps {
	criterionSelections: readonly CriterionSelection[];
	onBack(): void;
	symbol: string;
}

export function InstrumentAnalysisScreen({
	criterionSelections,
	onBack,
	symbol,
}: InstrumentAnalysisScreenProps) {
	const contentSpacing = useMatches({ base: "sm", sm: "md" });
	const paperPadding = useMatches({ base: "xs", sm: "md" });
	const textSize = useMatches({ base: "sm", sm: "md" });
	const permission = useBusinessRequestPermission();
	const hasNativeBackButton = useTelegramBackButton(onBack);
	const query = useInstrumentAnalysisQuery(
		symbol,
		criterionSelections,
		permission.allowed,
	);

	const insufficientHistory =
		query.error instanceof ApiError && query.error.code === "insufficient_data";
	useAnalysisErrorNotification(
		insufficientHistory ? null : query.error,
		"Instrument Analysis failed",
	);
	useAnalysisWarningNotification(
		query.data?.warnings,
		"Instrument Analysis warning",
	);

	const result = query.data;
	const marketCap = result && marketCapEvaluation(result.evaluations);

	const statistics = rangeStatistics.map((statistic) => {
		const { key } = statistic;
		const evaluation = result?.evaluations.find((item) => item.key === key);
		const period = criterionSelections.find((item) => item.key === key)
			?.parameters.period;
		const range = evaluation?.metrics[evaluationMetricKeys.rangePercent];
		const count = evaluation?.candle_count;
		const hasCoverage =
			typeof count === "number" &&
			Number.isInteger(count) &&
			count >= 0 &&
			typeof period === "number" &&
			Number.isInteger(period) &&
			period > 0;
		const coverage = hasCoverage
			? `${Math.min(count, period)} of ${period}`
			: "—";
		return {
			...statistic,
			range,
			coverage,
			partial: hasCoverage && count < period,
		};
	});

	return (
		<Container maw={720} px={0} size="sm">
			<Stack gap={contentSpacing}>
				{hasNativeBackButton ? null : (
					<Button onClick={onBack} size={textSize} variant="subtle">
						Back to Market Scan
					</Button>
				)}

				{query.isFetching && !query.data ? (
					<Center mih={180}>
						<Loader aria-label="Loading Instrument Analysis" />
					</Center>
				) : null}

				{result || insufficientHistory ? (
					<RefreshingOverlay
						label="Refreshing Instrument Analysis"
						visible={query.isFetching}
					>
						<Stack gap={contentSpacing}>
							<Paper p={paperPadding}>
								<Stack gap={contentSpacing}>
									<Group justify="space-between" wrap="nowrap">
										<Text size={textSize}>Symbol</Text>
										<Text fw={700} size={textSize} ta="right">
											{result?.symbol ?? symbol}
										</Text>
									</Group>
									{marketCap ? (
										<Group justify="space-between" wrap="nowrap">
											<Text size={textSize}>Market Cap</Text>
											<Text fw={700} size={textSize} ta="right">
												{formatMarketCapUsd(marketCap.marketCapUsd)}
											</Text>
										</Group>
									) : null}
									{statistics.map(
										({ key, label, coverageLabel, range, coverage }) => {
											return (
												<Stack gap={contentSpacing} key={key}>
													<Group justify="space-between" wrap="nowrap">
														<Text size={textSize}>{label}</Text>
														<Text fw={700} size={textSize} ta="right">
															{typeof range === "number" &&
															Number.isFinite(range)
																? formatRangePercent(range)
																: "—"}
														</Text>
													</Group>
													<Group justify="space-between" wrap="nowrap">
														<Text size={textSize}>{`${coverageLabel}: `}</Text>
														<Text fw={700} size={textSize} ta="right">
															{coverage}
														</Text>
													</Group>
												</Stack>
											);
										},
									)}
								</Stack>
							</Paper>
							<Paper
								component="section"
								aria-labelledby="bot-settings-heading"
								p={paperPadding}
							>
								<Stack gap="md">
									<Title id="bot-settings-heading" order={2} size="h3">
										Recommended trading bot settings
									</Title>
									<SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
										{statistics.map(
											({ key, stepLabel, range, coverage, partial }) => (
												<Stack gap={4} key={key}>
													<Text fw={500}>{stepLabel}</Text>
													<Text fw={700} size="xl">
														{typeof range === "number" && Number.isFinite(range)
															? formatRangePercent(range / 2)
															: "Not enough data"}
													</Text>
													{partial ? (
														<Text size="sm">Incomplete sample: {coverage}</Text>
													) : null}
												</Stack>
											),
										)}
									</SimpleGrid>
									<Text size="sm" c="dimmed">
										Grid Step is the percentage spacing between adjacent grid
										orders.
									</Text>
								</Stack>
							</Paper>
							{result ? (
								<Paper
									component="section"
									aria-labelledby="price-history-heading"
									p={paperPadding}
								>
									<Stack gap="md">
										<Title id="price-history-heading" order={2} size="h3">
											Seven-day Price History
										</Title>
										<PriceHistoryChart
											height={180}
											prices={result.price_history}
											responsive
											symbol={result.symbol}
											width={640}
											window={result.price_history_window}
										/>
									</Stack>
								</Paper>
							) : null}
						</Stack>
					</RefreshingOverlay>
				) : null}
			</Stack>
		</Container>
	);
}
