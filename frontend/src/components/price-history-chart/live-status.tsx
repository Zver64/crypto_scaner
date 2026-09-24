import { Group, Loader, Text } from "@mantine/core";
import type { PriceHistorySnapshot } from "@/components/price-history-chart/types";

type LiveStatusProps = Pick<
	PriceHistorySnapshot,
	"connection" | "freshness" | "error"
>;

export function LiveStatus({ connection, freshness, error }: LiveStatusProps) {
	if (!error && connection === "connected" && freshness === "fresh")
		return null;
	const label =
		error ??
		(connection === "disconnected"
			? "Backend connection lost — showing the last live candle"
			: freshness === "stale"
				? "Live stream interrupted — showing stale data"
				: freshness === "recovering"
					? "Recovering missed candle history…"
					: "Waiting for the current live candle…");
	return (
		<Group gap="xs" mb="xs" wrap="nowrap">
			{connection !== "disconnected" && freshness !== "stale" ? (
				<Loader size="xs" />
			) : null}
			<Text c="dimmed" size="xs">
				{label}
			</Text>
		</Group>
	);
}
