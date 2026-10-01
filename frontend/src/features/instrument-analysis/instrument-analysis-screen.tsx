import { Center, Container, Loader, Stack } from "@mantine/core";
import { useMemo } from "react";
import type { CriterionRequest } from "@/api/generated/models";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import { criterionKeys } from "@/features/analysis/identifiers";
import { CoinBackButton } from "@/features/instrument-analysis/coin-back-button";
import { CoinChart } from "@/features/instrument-analysis/coin-chart";
import { createCoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { CoinOverview } from "@/features/instrument-analysis/coin-overview";
import { CurrentPrice } from "@/features/instrument-analysis/current-price";
import { currentSevenDayHourlyCloses } from "@/features/instrument-analysis/hourly-history";
import { SpotGridEstimator } from "@/features/instrument-analysis/spot-grid-estimator/spot-grid-estimator";
import { useChartIndicators } from "@/features/instrument-analysis/use-chart-indicators";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";
import { useHourlyHistory } from "@/features/instrument-analysis/use-hourly-history";
import { useInstrumentAnalysis } from "@/features/instrument-analysis/use-instrument-analysis";
import { volatilityEvaluation } from "@/features/market-scan/criteria";
import { PriceAlertsPanel } from "@/features/price-alerts/price-alerts-panel";
import { sevenDayChangePercent } from "@/utils/seven-day-change-percent";

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
	const { contentSpacing, paperPadding } = useCoinPageLayout();
	const permission = useBusinessRequestPermission();
	const { insufficientHistory, isFetching, result } = useInstrumentAnalysis(
		symbol,
		criterionSelections,
		permission.allowed,
	);
	const hourlyHistory = useHourlyHistory(symbol, permission.allowed);
	const chart = useChartIndicators(permission.allowed);
	// Shared by the chart and the price alerts, which show its current price.
	const chartSource = useMemo(
		() => chart.catalogs && createCoinChartData(symbol, chart.catalogs),
		[chart.catalogs, symbol],
	);

	const sevenDayChange = result
		? sevenDayChangePercent(currentSevenDayHourlyCloses(hourlyHistory.candles))
		: null;
	// The previous symbol's result stays visible while refetching; the spot grid
	// only uses a result for this symbol.
	const current = result?.symbol === symbol ? result : undefined;
	const recommendationReady = current !== undefined && !hourlyHistory.pending;
	const rangePercent = (key: string) => {
		const range = current && volatilityEvaluation(current.evaluations, key);
		return range && Number.isFinite(range.rangePercent)
			? range.rangePercent
			: undefined;
	};

	return (
		<Container maw={720} px={0} size="sm">
			<Stack gap={contentSpacing}>
				<CoinBackButton onBack={onBack} />
				{isFetching && !result ? (
					<Center mih={180}>
						<Loader aria-label="Loading Instrument Analysis" />
					</Center>
				) : null}
				{result || insufficientHistory ? (
					<RefreshingOverlay
						label="Refreshing Instrument Analysis"
						visible={isFetching}
					>
						<Stack gap={contentSpacing}>
							<CoinOverview result={result} sevenDayChange={sevenDayChange} />
							{result ? (
								<CoinChart
									enabled={permission.allowed}
									failed={chart.failed}
									indicators={chart.indicators}
									source={chartSource}
									symbol={symbol}
								/>
							) : null}
							<SpotGridEstimator
								candles={
									recommendationReady ? hourlyHistory.candles : undefined
								}
								dailyVolatilityPercent={rangePercent(
									criterionKeys.dailyVolatility,
								)}
								disabled={isFetching || hourlyHistory.pending}
								hourlyVolatilityPercent={rangePercent(
									criterionKeys.hourlyVolatility,
								)}
								key={`binance:spot:USDT:${symbol}:${recommendationReady ? "ready" : "pending"}`}
								paperPadding={paperPadding}
							/>
						</Stack>
					</RefreshingOverlay>
				) : null}
				{permission.allowed ? (
					<PriceAlertsPanel
						currentPrice={
							chartSource ? <CurrentPrice source={chartSource} /> : null
						}
						priceSource={chartSource}
						symbol={symbol}
					/>
				) : null}
			</Stack>
		</Container>
	);
}
