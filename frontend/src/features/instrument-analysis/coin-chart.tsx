import { useMemo } from "react";
import { PriceHistoryChart } from "@/components/price-history-chart";
import { createCoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { CoinChartPlaceholder } from "@/features/instrument-analysis/coin-chart-placeholder";
import { rangeReadout } from "@/features/instrument-analysis/coin-chart-presentation";
import { useChartIndicators } from "@/features/instrument-analysis/use-chart-indicators";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";

interface CoinChartProps {
	enabled: boolean;
	symbol: string;
}

// The live chart waits for the backend indicator catalog, which defines both
// what it subscribes to and what it draws.
export function CoinChart({ enabled, symbol }: CoinChartProps) {
	const { paperPadding } = useCoinPageLayout();
	const { catalog, failed, indicators } = useChartIndicators(enabled);
	const source = useMemo(
		() => catalog && createCoinChartData(symbol, catalog),
		[catalog, symbol],
	);
	if (!source) {
		return <CoinChartPlaceholder failed={failed} paperPadding={paperPadding} />;
	}
	return (
		<PriceHistoryChart
			enabled={enabled}
			extraReadout={rangeReadout}
			indicators={indicators}
			key={symbol}
			paperPadding={paperPadding}
			source={source}
			symbol={symbol}
		/>
	);
}
