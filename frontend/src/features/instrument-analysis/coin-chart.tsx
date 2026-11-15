import {
	type ChartPriceLevels,
	PriceHistoryChart,
} from "@/features/candle-chart";
import type { CoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { CoinChartPlaceholder } from "@/features/instrument-analysis/coin-chart-placeholder";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";

interface CoinChartProps {
	enabled: boolean;
	failed: boolean;
	fillHeight: boolean;
	priceLevels?: ChartPriceLevels;
	source: CoinChartData | undefined;
	symbol: string;
}

// The live chart waits for its source, which waits for the backend indicator
// catalogs of every interval that define what it subscribes to.
export function CoinChart({
	enabled,
	failed,
	fillHeight,
	priceLevels,
	source,
	symbol,
}: CoinChartProps) {
	const { paperPadding } = useCoinPageLayout();
	if (!source) {
		return <CoinChartPlaceholder failed={failed} paperPadding={paperPadding} />;
	}
	return (
		<PriceHistoryChart
			comparison={source.comparison}
			enabled={enabled}
			fillHeight={fillHeight}
			key={symbol}
			paperPadding={paperPadding}
			priceLevels={priceLevels}
			source={source}
			symbol={symbol}
		/>
	);
}
