import { Text } from "@mantine/core";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { useTelegramBackButton } from "@/app/telegram";
import { useAdministrator } from "@/app/use-administrator";
import { ScannerSettings } from "@/features/scanner-settings/scanner-settings";
import { useCloseScannerSettings } from "@/features/scanner-settings/use-close-scanner-settings";

export function ScannerSettingsScreen() {
	const permission = useBusinessRequestPermission();
	const administrator = useAdministrator(permission.allowed);
	// Telegram's native back button leaves the settings like the header cross.
	useTelegramBackButton(useCloseScannerSettings());
	if (!administrator) {
		return (
			<Text c="dimmed" size="sm">
				Scanner settings are available only to the administrator.
			</Text>
		);
	}
	return <ScannerSettings />;
}
