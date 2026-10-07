import { createFileRoute } from "@tanstack/react-router";
import { ApiTokenSettings } from "@/features/api-token-settings/api-token-settings";

export const Route = createFileRoute("/admin/tokens")({
	component: ApiTokenSettings,
});
