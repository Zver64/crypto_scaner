import { createFileRoute } from "@tanstack/react-router";
import { StrategySettings } from "@/features/strategy-settings/strategy-settings";

export const Route = createFileRoute("/admin/signals")({
	component: () => <StrategySettings kind="signal" />,
});
