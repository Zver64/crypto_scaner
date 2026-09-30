import { createFileRoute } from "@tanstack/react-router";
import { StrategySettings } from "@/features/strategy-settings/strategy-settings";

export const Route = createFileRoute("/admin/strategies")({
	component: StrategySettings,
});
