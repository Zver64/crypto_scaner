import type { CandleInterval } from "@/api/generated/models";
import {
	type ChartIndicatorOptions,
	PriceHistoryChart,
} from "@/features/candle-chart";
import type { CoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { CoinChartPlaceholder } from "@/features/instrument-analysis/coin-chart-placeholder";
import { rangeReadout } from "@/features/instrument-analysis/coin-chart-presentation";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";

interface CoinChartProps {
	enabled: boolean;
	failed: boolean;
	indicators: Readonly<
		Record<CandleInterval, readonly ChartIndicatorOptions[]>
	>;
	source: CoinChartData | undefined;
	symbol: string;
}

// The live chart waits for the backend indicator catalogs of every interval,
// which define both what it subscribes to and what it draws.
export function CoinChart({
	enabled,
	failed,
	indicators,
	source,
	symbol,
}: CoinChartProps) {
	const { paperPadding } = useCoinPageLayout();
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
