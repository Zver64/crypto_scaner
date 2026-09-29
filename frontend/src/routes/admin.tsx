import { createFileRoute } from "@tanstack/react-router";
import { ScannerSettingsScreen } from "@/features/scanner-settings/scanner-settings-screen";

export const Route = createFileRoute("/admin")({
	beforeLoad: () => ({ pageTitle: "Scanner settings" }),
	component: ScannerSettingsScreen,
});
