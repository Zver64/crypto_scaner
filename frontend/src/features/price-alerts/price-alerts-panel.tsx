import type { ReactNode } from "react";
import { telegramUserScope } from "@/features/favorites/user-query-scope";
import type { CoinChartData } from "@/features/instrument-analysis/coin-chart-data";
import { PriceAlertsController } from "@/features/price-alerts/price-alerts-controller";

interface PriceAlertsPanelProps {
	// Shown next to the title.
	currentPrice?: ReactNode;
	// Live price the new target's percent change is measured from.
	priceSource?: CoinChartData;
	symbol: string;
}

export function PriceAlertsPanel({
	currentPrice,
	priceSource,
	symbol,
}: PriceAlertsPanelProps) {
	const scope = telegramUserScope();
	return (
		<PriceAlertsController
			currentPrice={currentPrice}
			key={`${scope}:${symbol}`}
			priceSource={priceSource}
			scope={scope}
			symbol={symbol}
		/>
	);
}
