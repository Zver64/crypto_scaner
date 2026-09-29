import { createFileRoute } from "@tanstack/react-router";
import { SettingsScreen } from "@/features/settings/settings-screen";

export const Route = createFileRoute("/admin")({
	beforeLoad: () => ({ pageTitle: "Settings" }),
	component: SettingsScreen,
});
