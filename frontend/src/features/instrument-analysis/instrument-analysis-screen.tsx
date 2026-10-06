import { Center, Loader, Stack } from "@mantine/core";
import { useMemo } from "react";
import type { CriterionRequest } from "@/api/generated/models";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import { SidebarLayout } from "@/components/sidebar-layout";
import { useSidebarLayoutActive } from "@/components/sidebar-layout/use-sidebar-layout-active";
import { criterionKeys } from "@/features/analysis/identifiers";
import { CoinBackButton } from "@/features/instrument-analysis/coin-back-button";
import { CoinChart } from "@/features/instrument-analysis/coin-chart";
import { createCoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { CurrentPrice } from "@/features/instrument-analysis/current-price";
import { GridCalculator } from "@/features/instrument-analysis/grid-estimator/grid-calculator";
import { useChartIndicators } from "@/features/instrument-analysis/use-chart-indicators";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";
import { useGridLimits } from "@/features/instrument-analysis/use-grid-limits";
import { useHourlyHistory } from "@/features/instrument-analysis/use-hourly-history";
import { useInstrumentAnalysis } from "@/features/instrument-analysis/use-instrument-analysis";
import { volatilityEvaluation } from "@/features/market-scan/criteria";
import { PriceAlertsPanel } from "@/features/price-alerts/price-alerts-panel";
import { baseAssetOfUsdtSymbol } from "@/utils/base-asset";

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
	// Beside the sidebar the page fits the viewport and the chart takes the
	// free height.
	const fillChart = useSidebarLayoutActive();
	const permission = useBusinessRequestPermission();
	const { insufficientHistory, isFetching, result } = useInstrumentAnalysis(
		symbol,
		criterionSelections,
		permission.allowed,
	);
	const hourlyHistory = useHourlyHistory(symbol, permission.allowed);
	const gridLimits = useGridLimits(symbol, permission.allowed);
	const chart = useChartIndicators(permission.allowed);
	// Shared by the chart and the price alerts, which show its current price.
	const chartSource = useMemo(
		() => chart.catalogs && createCoinChartData(symbol, chart.catalogs),
		[chart.catalogs, symbol],
	);

	// The previous symbol's result stays visible while refetching; the spot grid
	// only uses a result for this symbol.
	const current = result?.symbol === symbol ? result : undefined;
	const recommendationReady =
		current !== undefined && !hourlyHistory.pending && !gridLimits.pending;
	const rangePercent = (key: string) => {
		const range = current && volatilityEvaluation(current.evaluations, key);
		return range && Number.isFinite(range.rangePercent)
			? range.rangePercent
			: undefined;
	};

	return (
		<SidebarLayout
			desktop="fill"
			gap={contentSpacing}
			sidebar={
				<Stack gap={contentSpacing}>
					{result || insufficientHistory ? (
						<RefreshingOverlay
							label="Refreshing the grid calculator"
							visible={isFetching}
						>
							<GridCalculator
								baseAsset={baseAssetOfUsdtSymbol(symbol)}
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
								gridLimits={recommendationReady ? gridLimits.limits : undefined}
								key={`binance:spot:USDT:${symbol}:${recommendationReady ? "ready" : "pending"}`}
								paperPadding={paperPadding}
							/>
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
			}
			sidebarPosition="end"
		>
			<Stack flex={fillChart ? 1 : undefined} gap={contentSpacing}>
				<CoinBackButton onBack={onBack} />
				{isFetching && !result ? (
					<Center mih={180}>
						<Loader aria-label="Loading Instrument Analysis" />
					</Center>
				) : null}
				{result ? (
					<RefreshingOverlay
						fillHeight={fillChart}
						label="Refreshing Instrument Analysis"
						visible={isFetching}
					>
						<CoinChart
							enabled={permission.allowed}
							failed={chart.failed}
							fillHeight={fillChart}
							indicators={chart.indicators}
							source={chartSource}
							symbol={symbol}
						/>
					</RefreshingOverlay>
				) : null}
			</Stack>
		</SidebarLayout>
	);
}
