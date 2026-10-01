import type { ReactNode } from "react";
import { telegramUserScope } from "@/features/favorites/user-query-scope";
import { PriceAlertsController } from "@/features/price-alerts/price-alerts-controller";

interface PriceAlertsPanelProps {
	// Shown next to the title.
	currentPrice?: ReactNode;
	symbol: string;
}

export function PriceAlertsPanel({
	currentPrice,
	symbol,
}: PriceAlertsPanelProps) {
	const scope = telegramUserScope();
	return (
		<PriceAlertsController
			currentPrice={currentPrice}
			key={`${scope}:${symbol}`}
			scope={scope}
			symbol={symbol}
		/>
	);
}
