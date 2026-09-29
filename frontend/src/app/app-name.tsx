import { Text, UnstyledButton } from "@mantine/core";
import { IconX } from "@tabler/icons-react";
import { Link, useMatchRoute } from "@tanstack/react-router";
import { useCloseScannerSettings } from "@/features/scanner-settings/use-close-scanner-settings";

interface AppNameProps {
	administrator: boolean;
}

// The app name in the header. For the administrator it is an unmarked entry
// to the scanner settings and turns into a close button while they are open.
export function AppName({ administrator }: AppNameProps) {
	const matchRoute = useMatchRoute();
	const closeSettings = useCloseScannerSettings();
	const name = (
		<Text fw={800} lts="0.08em">
			CS
		</Text>
	);
	if (!administrator) {
		return name;
	}
	if (matchRoute({ fuzzy: true, to: "/admin" })) {
		return (
			<UnstyledButton
				aria-label="Close scanner settings"
				display="flex"
				onClick={closeSettings}
			>
				<IconX size={20} stroke={2.5} />
			</UnstyledButton>
		);
	}
	return (
		<Link style={{ color: "inherit", textDecoration: "none" }} to="/admin">
			{name}
		</Link>
	);
}
