import { Box, Text } from "@mantine/core";
import { Outlet, useLocation } from "@tanstack/react-router";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { useTelegramBackButton } from "@/app/telegram";
import { useAdministrator } from "@/app/use-administrator";
import { PageStack } from "@/components/page-stack";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";
import { useCloseScannerSettings } from "@/features/scanner-settings/use-close-scanner-settings";
import { SettingsNavigation } from "@/features/settings/settings-navigation";

// The administrator settings with a switch between their pages.
export function SettingsScreen() {
	const permission = useBusinessRequestPermission();
	const administrator = useAdministrator(permission.allowed);
	const closeSettings = useCloseScannerSettings();
	const pathname = useLocation({ select: (location) => location.pathname });
	// On wide screens the settings fit the viewport: the section switch stays
	// in view and the section scrolls, or fills the height, below it.
	const fitViewport = useWideLayout();
	// Telegram's native back button leaves the settings like the header cross.
	useTelegramBackButton(closeSettings);
	if (!administrator) {
		return (
			<Text c="dimmed" size="sm">
				Settings are available only to the administrator.
			</Text>
		);
	}
	return (
		<PageStack gap="md">
			<SettingsNavigation />
			<Box
				// The router restores only the window scroll; a new section starts
				// this scroll box at the top.
				key={pathname}
				style={
					fitViewport
						? {
								display: "flex",
								flex: 1,
								flexDirection: "column",
								minHeight: 0,
								overflowY: "auto",
							}
						: undefined
				}
			>
				<Outlet />
			</Box>
		</PageStack>
	);
}
