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

// The live chart waits for the backend indicator catalogs of every interval,
// which define both what it subscribes to and what it draws.
export function CoinChart({ enabled, symbol }: CoinChartProps) {
	const { paperPadding } = useCoinPageLayout();
	const { catalogs, failed, indicators } = useChartIndicators(enabled);
	const source = useMemo(
		() => catalogs && createCoinChartData(symbol, catalogs),
		[catalogs, symbol],
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
