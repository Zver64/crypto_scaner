import { useNavigate, useSearch } from "@tanstack/react-router";

// On a backtest coin page, returns to the coin list with its sort, filter and
// the chosen strategy. Replacing keeps a single settings history entry, so
// closing the settings still returns to the page they were opened from.
export function useBackToBacktestCoins(): (() => void) | undefined {
	const navigate = useNavigate();
	const search = useSearch({
		from: "/admin/backtest/$symbol",
		shouldThrow: false,
	});
	if (!search) return undefined;
	return () => void navigate({ replace: true, search, to: "/admin/backtest" });
}
