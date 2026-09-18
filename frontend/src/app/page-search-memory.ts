import type { MarketScanSearch } from "@/routes/-market-scan-search";
import type { TopCoinsSearch } from "@/routes/-top-coins-search";

type PageSearchEntry =
	| [path: "/", search: MarketScanSearch]
	| [path: "/top-coins", search: TopCoinsSearch];

export function createPageSearchMemory() {
	let marketScanSearch: MarketScanSearch | undefined;
	let topCoinsSearch: TopCoinsSearch | undefined;

	function remember(...entry: PageSearchEntry) {
		if (entry[0] === "/") {
			marketScanSearch = { ...entry[1] };
			return;
		}

		topCoinsSearch = { ...entry[1] };
	}

	function recall(path: "/"): MarketScanSearch | undefined;
	function recall(path: "/top-coins"): TopCoinsSearch | undefined;
	function recall(path: "/" | "/top-coins") {
		const search = path === "/" ? marketScanSearch : topCoinsSearch;
		return search ? { ...search } : undefined;
	}

	return { recall, remember };
}

export const pageSearchMemory = createPageSearchMemory();
