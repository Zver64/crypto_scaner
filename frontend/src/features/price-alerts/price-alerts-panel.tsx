import type { ReactNode } from "react";

import { telegramUserScope } from "@/features/favorites/user-query-scope";
import { PriceAlertsController } from "@/features/price-alerts/price-alerts-controller";
import type { CurrentPriceSource } from "@/features/price-alerts/types";

interface PriceAlertsPanelProps {
	// Shown next to the title.
	currentPrice?: ReactNode;
	// Live price the new target's percent change is measured from.
	priceSource: CurrentPriceSource;
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
