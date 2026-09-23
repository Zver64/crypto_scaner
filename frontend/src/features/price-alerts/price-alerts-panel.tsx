import { telegramUserScope } from "@/features/favorites/user-query-scope";
import { PriceAlertsController } from "@/features/price-alerts/price-alerts-controller";

export function PriceAlertsPanel({ symbol }: { symbol: string }) {
	const scope = telegramUserScope();
	return (
		<PriceAlertsController
			key={`${scope}:${symbol}`}
			scope={scope}
			symbol={symbol}
		/>
	);
}
