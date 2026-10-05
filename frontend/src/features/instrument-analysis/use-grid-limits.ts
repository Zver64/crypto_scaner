import { useGetInstrumentGridLimits } from "@/api/generated/api";
import type { SpotGridLimits } from "@/features/instrument-analysis/grid-estimator/types";

// Binance spot grid limits, fetched once per coin page so the calculator's
// range does not move while it is being edited. A failure leaves the
// calculator on the latest hourly close rather than blocking it.
export function useGridLimits(symbol: string, enabled: boolean) {
	const query = useGetInstrumentGridLimits(symbol, {
		query: {
			enabled,
			gcTime: 0,
			refetchOnReconnect: false,
			refetchOnWindowFocus: false,
			retry: false,
			staleTime: Number.POSITIVE_INFINITY,
			select: ({ data }): SpotGridLimits | null =>
				data.symbol === symbol.toUpperCase()
					? {
							askMultiplierUp: data.ask_multiplier_up,
							averagePrice: data.average_price,
							bidMultiplierDown: data.bid_multiplier_down,
							maxPrice: data.max_price,
							minPrice: data.min_price,
							tickSize: data.tick_size,
						}
					: null,
		},
	});
	return {
		limits: query.isError ? null : (query.data ?? null),
		pending: query.isPending && !query.isError,
	};
}
