import { Center, Loader, Stack } from "@mantine/core";

import { RefreshingOverlay } from "@/components/refreshing-overlay";
import { SidebarLayout } from "@/components/sidebar-layout";
import { sidebarColumns } from "@/components/sidebar-layout/constants";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";
import { VolatilitySettingsForm } from "@/features/analysis/volatility-settings-form";
import type { VolatilitySettings } from "@/features/analysis/volatility-settings-form/types";
import { CoinBackButton } from "@/features/instrument-analysis/coin-back-button";
import { CoinChart } from "@/features/instrument-analysis/coin-chart";
import { CurrentPrice } from "@/features/instrument-analysis/current-price";
import { GridCalculator } from "@/features/instrument-analysis/grid-estimator/grid-calculator";
import { useCoinPage } from "@/features/instrument-analysis/use-coin-page";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";
import { PriceAlertsPanel } from "@/features/price-alerts/price-alerts-panel";
import { baseAssetOfUsdtSymbol } from "@/utils/base-asset";

interface InstrumentAnalysisScreenProps {
	onBack(): void;
	onSettingsCommit(settings: VolatilitySettings): void;
	settings: VolatilitySettings;
	symbol: string;
}

export function InstrumentAnalysisScreen({
	onBack,
	onSettingsCommit,
	settings,
	symbol,
}: InstrumentAnalysisScreenProps) {
	const { contentSpacing, paperPadding } = useCoinPageLayout();
	// Wide screens put the chart beside a sidebar of the other blocks, and the
	// chart takes the free height; narrow ones stack every block, the settings
	// above the chart.
	const wide = useWideLayout();
	const page = useCoinPage(symbol, settings);
	const { insufficientHistory, isFetching, result } = page.analysis;

	const back = <CoinBackButton onBack={onBack} />;
	const settingsForm = (
		<VolatilitySettingsForm
			columns={sidebarColumns}
			disabled={!page.allowed}
			initialSettings={settings}
			loading={isFetching}
			onCommit={onSettingsCommit}
		/>
	);
	const chart = (
		<>
			{isFetching && !result ? (
				<Center mih={180}>
					<Loader aria-label="Loading Instrument Analysis" />
				</Center>
			) : null}
			{result ? (
				<RefreshingOverlay
					fillHeight={wide}
					label="Refreshing Instrument Analysis"
					visible={isFetching}
				>
					<CoinChart
						enabled={page.allowed}
						failed={page.indicators.failed}
						fillHeight={wide}
						indicators={page.indicators.indicators}
						priceLevels={page.gridLevels ?? undefined}
						source={page.chartSource}
						symbol={symbol}
					/>
				</RefreshingOverlay>
			) : null}
		</>
	);
	const calculator =
		result || insufficientHistory ? (
			<RefreshingOverlay
				label="Refreshing the grid calculator"
				visible={isFetching}
			>
				<GridCalculator
					{...page.calculator}
					baseAsset={baseAssetOfUsdtSymbol(symbol)}
					key={page.calculatorKey}
					onPriceLevelsChange={page.setGridLevels}
					paperPadding={paperPadding}
				/>
			</RefreshingOverlay>
		) : null;
	const alerts = page.allowed ? (
		<PriceAlertsPanel
			currentPrice={
				page.chartSource ? <CurrentPrice source={page.chartSource} /> : null
			}
			priceSource={page.chartSource}
			symbol={symbol}
		/>
	) : null;

	if (!wide) {
		return (
			<Stack gap={contentSpacing}>
				{back}
				{settingsForm}
				{chart}
				{calculator}
				{alerts}
			</Stack>
		);
	}
	return (
		<SidebarLayout
			gap={contentSpacing}
			sidebar={
				<Stack gap={contentSpacing}>
					{settingsForm}
					{calculator}
					{alerts}
				</Stack>
			}
			sidebarPosition="end"
		>
			<Stack flex={1} gap={contentSpacing}>
				{back}
				{chart}
			</Stack>
		</SidebarLayout>
	);
}
