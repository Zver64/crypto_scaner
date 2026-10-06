import type { UnresolvedInstrumentCode } from "@/api/generated/models";

export const marketCapUnavailableReasons: Record<
	UnresolvedInstrumentCode,
	string
> = {
	mapping_conflict:
		"Multiple market capitalization matches were found for this instrument.",
	mapping_not_found:
		"No market capitalization match was found for this instrument.",
	mapping_provider_unavailable:
		"Market capitalization mapping data is temporarily unavailable.",
	market_cap_missing:
		"Market capitalization data is unavailable for this instrument.",
};
