import {
	type ChartPriceLevels,
	PriceHistoryChart,
} from "@/features/candle-chart";
import type { CoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { CoinChartPlaceholder } from "@/features/instrument-analysis/coin-chart-placeholder";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";

interface CoinChartProps {
	failed: boolean;
	fillHeight: boolean;
	priceLevels?: ChartPriceLevels;
	// The backend indicator catalogs of every interval have loaded.
	ready: boolean;
	source: CoinChartData;
	symbol: string;
}

// The live chart waits for the backend indicator catalogs of every interval,
// which define the indicators and panes it draws.
export function CoinChart({
	failed,
	fillHeight,
	priceLevels,
	ready,
	source,
	symbol,
}: CoinChartProps) {
	const { paperPadding } = useCoinPageLayout();
	if (!ready) {
		return <CoinChartPlaceholder failed={failed} paperPadding={paperPadding} />;
	}
	return (
		<PriceHistoryChart
			comparison={source.comparison}
			fillHeight={fillHeight}
			key={symbol}
			paperPadding={paperPadding}
			priceLevels={priceLevels}
			source={source}
			symbol={symbol}
		/>
	);
}
