import { createFileRoute } from "@tanstack/react-router";
import { ScannerSettings } from "@/features/scanner-settings/scanner-settings";

export const Route = createFileRoute("/admin/")({
	component: ScannerSettings,
});
