import {
	getInstrumentGridLimits,
	type getInstrumentGridLimitsResponseSuccess,
	useGetInstrumentGridLimits,
} from "@/api/generated/api";
import { unexpectedApiError } from "@/utils/api-error";

// Binance grid limits, fetched once per coin page so the calculator's
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
			select: gridLimits,
			staleTime: Number.POSITIVE_INFINITY,
		},
	});
	return {
		limits: query.isError ? null : (query.data ?? null),
		pending: query.isPending && !query.isError,
	};
}

// Defined at module level so select runs only when the response changes.
function gridLimits({ data }: getInstrumentGridLimitsResponseSuccess) {
	return data;
}
