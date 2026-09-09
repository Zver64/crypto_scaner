import { createFileRoute } from "@tanstack/react-router";
import { TopCoinsScreen } from "@/features/top-coins/top-coins-screen";

export const Route = createFileRoute("/top-coins")({
	component: TopCoinsScreen,
});
