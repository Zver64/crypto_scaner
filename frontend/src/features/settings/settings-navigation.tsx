import { SegmentedControl } from "@mantine/core";
import { useMatchRoute, useNavigate } from "@tanstack/react-router";
import { useBackToBacktestCoins } from "@/features/strategy-backtest/use-back-to-backtest-coins";

const pages = [
	{ label: "Indicators", value: "/admin" },
	{ label: "Strategies", value: "/admin/strategies" },
	{ label: "Backtest", value: "/admin/backtest" },
	{ label: "Users", value: "/admin/users" },
] as const;

type SettingsPage = (typeof pages)[number]["value"];

export function SettingsNavigation() {
	const matchRoute = useMatchRoute();
	const navigate = useNavigate();
	const backToCoins = useBackToBacktestCoins();
	// A backtest coin page selects no section, so choosing Backtest there
	// returns to its coin list, like its back button.
	const current: SettingsPage | null = matchRoute({ to: "/admin/users" })
		? "/admin/users"
		: matchRoute({ to: "/admin/strategies" })
			? "/admin/strategies"
			: matchRoute({ to: "/admin/backtest" })
				? "/admin/backtest"
				: matchRoute({ to: "/admin/backtest/$symbol" })
					? null
					: "/admin";
	return (
		<SegmentedControl
			data={pages.map(({ label, value }) => ({ label, value }))}
			fullWidth
			onChange={(value) => {
				const page = pages.find((item) => item.value === value);
				// Replacing keeps a single history entry, so closing the settings
				// returns to the page they were opened from.
				if (!page) return;
				if (page.value === "/admin/backtest" && backToCoins) backToCoins();
				else void navigate({ replace: true, to: page.value });
			}}
			value={current ?? ""}
		/>
	);
}
