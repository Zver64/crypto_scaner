import { ScrollArea, SegmentedControl } from "@mantine/core";
import { useLocation, useNavigate } from "@tanstack/react-router";

const pages = [
	{ label: "Indicators", value: "/admin" },
	{ label: "Signals", value: "/admin/signals" },
	{ label: "Strategies", value: "/admin/strategies" },
	{ label: "Backtest", value: "/admin/backtest" },
	{ label: "Users", value: "/admin/users" },
	{ label: "Tokens", value: "/admin/tokens" },
	{ label: "Commands", value: "/admin/commands" },
] as const;

type SettingsPage = (typeof pages)[number]["value"];

export function SettingsNavigation() {
	// The pathname, unlike the stable useMatchRoute callback, changes on
	// navigation, so React Compiler recomputes the current page.
	const pathname = useLocation({ select: (location) => location.pathname });
	const navigate = useNavigate();
	const current: SettingsPage =
		pages.find(
			({ value }) =>
				value !== "/admin" &&
				(pathname === value || pathname.startsWith(`${value}/`)),
		)?.value ?? "/admin";
	// The switch fills the width and scrolls sideways on phones too narrow
	// for every section.
	return (
		<ScrollArea scrollbars="x" type="never">
			<SegmentedControl
				data={pages.map(({ label, value }) => ({ label, value }))}
				fullWidth
				miw="max-content"
				p={4}
				radius="md"
				size="md"
				withItemsBorders={false}
				onChange={(value) => {
					const page = pages.find((item) => item.value === value);
					// Replacing keeps a single history entry, so closing the settings
					// returns to the page they were opened from.
					if (!page) return;
					void navigate({ replace: true, to: page.value });
				}}
				value={current}
			/>
		</ScrollArea>
	);
}
