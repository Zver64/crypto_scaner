import { SegmentedControl } from "@mantine/core";
import { useMatchRoute, useNavigate } from "@tanstack/react-router";

const pages = [
	{ label: "Indicators", value: "/admin" },
	{ label: "Strategies", value: "/admin/strategies" },
	{ label: "Users", value: "/admin/users" },
] as const;

type SettingsPage = (typeof pages)[number]["value"];

export function SettingsNavigation() {
	const matchRoute = useMatchRoute();
	const navigate = useNavigate();
	const current: SettingsPage = matchRoute({ to: "/admin/users" })
		? "/admin/users"
		: matchRoute({ to: "/admin/strategies" })
			? "/admin/strategies"
			: "/admin";
	return (
		<SegmentedControl
			data={pages.map(({ label, value }) => ({ label, value }))}
			fullWidth
			onChange={(value) => {
				const page = pages.find((item) => item.value === value);
				// Replacing keeps a single history entry, so closing the settings
				// returns to the page they were opened from.
				if (page) void navigate({ replace: true, to: page.value });
			}}
			value={current}
		/>
	);
}
