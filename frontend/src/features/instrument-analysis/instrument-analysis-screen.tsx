import {
	Button,
	Center,
	Container,
	Group,
	Loader,
	Paper,
	Stack,
	Text,
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
import { PercentChange } from "@/components/percent-change";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import { useAnalysisErrorNotification } from "@/features/analysis/use-analysis-error-notification";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import { InstrumentPriceHistoryChart } from "@/features/instrument-analysis/price-history-chart";
import { SpotGridEstimator } from "@/features/instrument-analysis/spot-grid-estimator/spot-grid-estimator";
import { formatMarketCapUsd, marketCapEvaluation } from "@/utils/market-cap";
import { formatRangePercent } from "@/utils/range-percent";
import { sevenDayChangePercent } from "@/utils/seven-day-change-percent";

const rangeStatistics = [
	{
		key: criterionKeys.dailyVolatility,
		label: "Daily Range",
		coverageLabel: "Days available",
	},
	{
		key: criterionKeys.hourlyVolatility,
		label: "Hourly Range",
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
	const sevenDayChange = result
		? sevenDayChangePercent(
				result.candle_history
					.slice(-169)
					.map((candle) => candle?.close ?? null),
			)
		: null;

	const recommendationResult = result?.symbol === symbol ? result : undefined;
	const hourlyRange = recommendationResult?.evaluations.find(
		(item) => item.key === criterionKeys.hourlyVolatility,
	)?.metrics[evaluationMetricKeys.rangePercent];
	const hourlyVolatilityPercent =
		typeof hourlyRange === "number" && Number.isFinite(hourlyRange)
			? hourlyRange
			: undefined;

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
							{result ? (
								<Paper
									component="section"
									aria-labelledby="price-history-heading"
									p={paperPadding}
								>
									<Stack gap="md">
										<InstrumentPriceHistoryChart
											candles={result.candle_history}
											symbol={result.symbol}
											window={result.price_history_window}
										/>
									</Stack>
								</Paper>
							) : null}
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
									{result ? (
										<Group justify="space-between" wrap="nowrap">
											<Text size={textSize}>7d change percent</Text>
											<Text fw={700} size={textSize} ta="right">
												<PercentChange value={sevenDayChange} />
											</Text>
										</Group>
									) : null}
									{statistics.map(({ key, label, range }) => (
										<Group justify="space-between" key={key} wrap="nowrap">
											<Text size={textSize}>{label}</Text>
											<Text fw={700} size={textSize} ta="right">
												{typeof range === "number" && Number.isFinite(range)
													? formatRangePercent(range)
													: "—"}
											</Text>
										</Group>
									))}
									{statistics.map(({ key, coverageLabel, coverage }) => (
										<Group
											justify="space-between"
											key={`${key}-coverage`}
											wrap="nowrap"
										>
											<Text size={textSize}>{`${coverageLabel}: `}</Text>
											<Text fw={700} size={textSize} ta="right">
												{coverage}
											</Text>
										</Group>
									))}
								</Stack>
							</Paper>
							<SpotGridEstimator
								candles={recommendationResult?.candle_history}
								disabled={query.isFetching}
								hourlyVolatilityPercent={hourlyVolatilityPercent}
								key={`binance:spot:USDT:${symbol}:${recommendationResult ? "ready" : "pending"}`}
								paperPadding={paperPadding}
							/>
						</Stack>
					</RefreshingOverlay>
				) : null}
			</Stack>
		</Container>
	);
}
