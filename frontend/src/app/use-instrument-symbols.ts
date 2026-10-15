import {
	type listInstrumentsResponseSuccess,
	useListInstruments,
} from "@/api/generated/api";
import type { MarketUniverse } from "@/api/generated/models";
import { useBusinessRequestPermission } from "@/app/business-request-context";

const selectSymbols = (response: listInstrumentsResponseSuccess) =>
	response.data.items.map(({ symbol }) => symbol).sort();

// The active coins of universe, every active coin by default, ascending;
// loaded only while enabled.
export function useInstrumentSymbols(
	universe?: MarketUniverse,
	enabled = true,
) {
	const permission = useBusinessRequestPermission();
	return useListInstruments(universe === undefined ? undefined : { universe }, {
		query: {
			enabled: permission.allowed && enabled,
			retry: false,
			select: selectSymbols,
		},
	});
}
