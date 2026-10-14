import {
	getInstrumentGridLimits,
	type getInstrumentGridLimitsResponseSuccess,
	useGetInstrumentGridLimits,
} from "@/api/generated/api";
import { unexpectedApiError } from "@/features/analysis/api-error";
import type { SpotGridLimits } from "@/features/instrument-analysis/grid-estimator/types";

// Binance spot grid limits, fetched once per coin page so the calculator's
// range does not move while it is being edited. A failure, including limits
// for another symbol, leaves the calculator on the latest hourly close rather
// than blocking it.
export function useGridLimits(symbol: string, enabled: boolean) {
	const query = useGetInstrumentGridLimits(symbol, {
		query: {
			enabled,
			gcTime: 0,
			// Validated when fetched, so limits for another symbol fail the query.
			queryFn: async ({ signal }) => {
				const response = await getInstrumentGridLimits(symbol, { signal });
				if (response.data.symbol !== symbol.toUpperCase()) {
					throw unexpectedApiError();
				}
				return response;
			},
			refetchOnReconnect: false,
			refetchOnWindowFocus: false,
			retry: false,
			select: spotGridLimits,
			staleTime: Number.POSITIVE_INFINITY,
		},
	});
	return {
		limits: query.isError ? null : (query.data ?? null),
		pending: query.isPending && !query.isError,
	};
}

// Defined at module level so select runs only when the response changes.
function spotGridLimits({
	data,
}: getInstrumentGridLimitsResponseSuccess): SpotGridLimits {
	return {
		askMultiplierUp: data.ask_multiplier_up,
		averagePrice: data.average_price,
		bidMultiplierDown: data.bid_multiplier_down,
		maxPrice: data.max_price,
		minPrice: data.min_price,
		tickSize: data.tick_size,
	};
}
