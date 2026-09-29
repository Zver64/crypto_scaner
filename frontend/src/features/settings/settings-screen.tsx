import { Stack, Text } from "@mantine/core";
import { Outlet } from "@tanstack/react-router";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { useTelegramBackButton } from "@/app/telegram";
import { useAdministrator } from "@/app/use-administrator";
import { useCloseScannerSettings } from "@/features/scanner-settings/use-close-scanner-settings";
import { SettingsNavigation } from "@/features/settings/settings-navigation";

// The administrator settings with a switch between their pages.
export function SettingsScreen() {
	const permission = useBusinessRequestPermission();
	const administrator = useAdministrator(permission.allowed);
	// Telegram's native back button leaves the settings like the header cross.
	useTelegramBackButton(useCloseScannerSettings());
	if (!administrator) {
		return (
			<Text c="dimmed" size="sm">
				Settings are available only to the administrator.
			</Text>
		);
	}
	return (
		<Stack gap="md">
			<SettingsNavigation />
			<Outlet />
		</Stack>
	);
}
