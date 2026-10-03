import { useGetInstrumentPriceLimits } from "@/api/generated/api";
import type { SpotGridPriceLimits } from "@/features/instrument-analysis/spot-grid-estimator/utils";

// Binance price limits for the spot grid, fetched once per coin page so the
// calculator's range does not move while it is being edited. A failure leaves
// the calculator without limits rather than blocking it.
export function usePriceLimits(symbol: string, enabled: boolean) {
	const query = useGetInstrumentPriceLimits(symbol, {
		query: {
			enabled,
			gcTime: 0,
			refetchOnReconnect: false,
			refetchOnWindowFocus: false,
			retry: false,
			staleTime: Number.POSITIVE_INFINITY,
			select: ({ data }): SpotGridPriceLimits | null =>
				data.symbol === symbol.toUpperCase()
					? {
							askLimitMultUp: data.ask_limit_mult_up,
							bidLimitMultDown: data.bid_limit_mult_down,
							referencePrice: data.reference_price,
						}
					: null,
		},
	});
	return {
		limits: query.isError ? null : (query.data ?? null),
		pending: query.isPending && !query.isError,
	};
}
