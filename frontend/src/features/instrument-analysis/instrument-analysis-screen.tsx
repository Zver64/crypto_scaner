import {
	Button,
	Center,
	Container,
	Group,
	Loader,
	Paper,
	Stack,
	Text,
	useMantineTheme,
	useMatches,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import {
	type InfiniteData,
	keepPreviousData,
	useQueryClient,
} from "@tanstack/react-query";
import { useEffect, useMemo } from "react";
import {
	useAnalyzeInstrument,
	useListInstrumentCandlesInfinite,
} from "@/api/generated/api";
import type {
	CandlePageResponse,
	CriterionRequest,
	InstrumentAnalysisResponse,
} from "@/api/generated/models";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { telegramRequestOptions, useTelegramBackButton } from "@/app/telegram";
import { PercentChange } from "@/components/percent-change";
import { PriceHistoryChart } from "@/components/price-history-chart";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import {
	apiErrorCode,
	apiErrorMessage,
	unexpectedApiError,
} from "@/features/analysis/api-error";
import {
	criterionKeys,
	evaluationMetricKeys,
} from "@/features/analysis/identifiers";
import { hasExpectedInstrumentAnalysisEvaluations } from "@/features/analysis/semantics";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import {
	nextCandlePageParam,
	validateCandlePage,
} from "@/features/instrument-analysis/candle-page";
import { createCoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import {
	coinChartIntervals,
	formatCoinChartTime,
	nextCoinCandleOpen,
	rangeReadout,
	rsiIndicator,
} from "@/features/instrument-analysis/coin-chart-presentation";
import { currentSevenDayHourlyCloses } from "@/features/instrument-analysis/hourly-history";
import { SpotGridEstimator } from "@/features/instrument-analysis/spot-grid-estimator/spot-grid-estimator";
import { PriceAlertsPanel } from "@/features/price-alerts/price-alerts-panel";
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
	criterionSelections: readonly CriterionRequest[];
	onBack(): void;
	symbol: string;
}

export function InstrumentAnalysisScreen({
	criterionSelections,
	onBack,
	symbol,
}: InstrumentAnalysisScreenProps) {
	const theme = useMantineTheme();
	const contentSpacing = useMatches({ base: "sm", sm: "md" });
	const paperPadding = useMatches({ base: "xs", sm: "md" });
	const textSize = useMatches({ base: "sm", sm: "md" });
	const permission = useBusinessRequestPermission();
	const queryClient = useQueryClient();
	const chartSource = useMemo(
		() => createCoinChartData(symbol, queryClient),
		[symbol, queryClient],
	);
	// These hourly candles belong to the page's 7d/grid calculations, not the chart.
	const hourlyHistoryQuery = useListInstrumentCandlesInfinite<
		InfiniteData<CandlePageResponse, string | undefined>
	>(
		symbol,
		{ interval: "1h", limit: 200 },
		{
			fetch: telegramRequestOptions(),
			query: {
				enabled: permission.allowed,
				getNextPageParam: nextCandlePageParam,
				initialPageParam: undefined,
				retry: false,
				staleTime: Number.POSITIVE_INFINITY,
				select: (history) => ({
					...history,
					pages: history.pages.map((page) =>
						validateCandlePage(page, symbol, "1h"),
					),
				}),
			},
		},
	);
	const hourlyCandles = useMemo(() => {
		const data = hourlyHistoryQuery.data;
		return data
			? [...data.pages].reverse().flatMap((page) => page.candles)
			: [];
	}, [hourlyHistoryQuery.data]);
	const hasNativeBackButton = useTelegramBackButton(onBack);
	const query = useAnalyzeInstrument<InstrumentAnalysisResponse>(
		symbol,
		{ criteria: [...criterionSelections] },
		{
			fetch: telegramRequestOptions(),
			query: {
				enabled: permission.allowed,
				placeholderData: keepPreviousData,
				refetchOnMount: "always",
				retry: false,
				staleTime: 0,
				select: (response) => {
					if (
						!hasExpectedInstrumentAnalysisEvaluations(
							response.data.evaluations,
							criterionSelections,
						)
					) {
						throw unexpectedApiError();
					}
					return response.data;
				},
			},
		},
	);

	const insufficientHistory =
		query.isError && apiErrorCode(query.error) === "insufficient_data";
	useEffect(() => {
		if (query.isError && apiErrorCode(query.error) !== "insufficient_data") {
			notifications.show({
				id: `instrument-analysis-${symbol}-error`,
				autoClose: 5000,
				color: "red",
				message: apiErrorMessage(query.error),
				title: "Instrument Analysis failed",
			});
		}
	}, [query.error, query.isError, symbol]);
	useEffect(() => {
		if (hourlyHistoryQuery.isError) {
			notifications.show({
				id: `hourly-price-history-${symbol}-error`,
				autoClose: 5000,
				color: "red",
				message: apiErrorMessage(hourlyHistoryQuery.error),
				title: "Hourly price history failed",
			});
		}
	}, [hourlyHistoryQuery.error, hourlyHistoryQuery.isError, symbol]);
	useAnalysisWarningNotification(
		query.data?.warnings,
		"Instrument Analysis warning",
	);

	const result = query.data;
	const marketCap = result && marketCapEvaluation(result.evaluations);
	const sevenDayChange = result
		? sevenDayChangePercent(currentSevenDayHourlyCloses(hourlyCandles))
		: null;

	const recommendationResult = result?.symbol === symbol ? result : undefined;
	const hourlyHistoryPending = hourlyHistoryQuery.isPending;
	const recommendationReady =
		recommendationResult !== undefined && !hourlyHistoryPending;
	const hourlyRange = recommendationResult?.evaluations.find(
		(item) => item.key === criterionKeys.hourlyVolatility,
	)?.metrics[evaluationMetricKeys.rangePercent];
	const dailyRange = recommendationResult?.evaluations.find(
		(item) => item.key === criterionKeys.dailyVolatility,
	)?.metrics[evaluationMetricKeys.rangePercent];
	const hourlyVolatilityPercent =
		typeof hourlyRange === "number" && Number.isFinite(hourlyRange)
			? hourlyRange
			: undefined;
	const dailyVolatilityPercent =
		typeof dailyRange === "number" && Number.isFinite(dailyRange)
			? dailyRange
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
											<Text
												c={theme.colors[theme.primaryColor][4]}
												fw={700}
												size={textSize}
												ta="right"
											>
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
							{result ? (
								<PriceHistoryChart
									enabled={permission.allowed}
									intervals={coinChartIntervals}
									indicator={rsiIndicator}
									formatTime={formatCoinChartTime}
									nextOpen={nextCoinCandleOpen}
									extraReadout={rangeReadout}
									key={symbol}
									paperPadding={paperPadding}
									source={chartSource}
									symbol={symbol}
								/>
							) : null}
							<SpotGridEstimator
								candles={recommendationReady ? hourlyCandles : undefined}
								dailyVolatilityPercent={dailyVolatilityPercent}
								disabled={query.isFetching || hourlyHistoryPending}
								hourlyVolatilityPercent={hourlyVolatilityPercent}
								key={`binance:spot:USDT:${symbol}:${recommendationReady ? "ready" : "pending"}`}
								paperPadding={paperPadding}
							/>
						</Stack>
					</RefreshingOverlay>
				) : null}
				{permission.allowed ? <PriceAlertsPanel symbol={symbol} /> : null}
			</Stack>
		</Container>
	);
}
