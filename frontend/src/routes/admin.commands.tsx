import { createFileRoute } from "@tanstack/react-router";
import { HistoryLoadCommand } from "@/features/command-settings/history-load-command";

export const Route = createFileRoute("/admin/commands")({
	component: HistoryLoadCommand,
});
