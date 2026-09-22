import { Group, Loader, Text } from "@mantine/core";
import type { LiveCandleServerMessageFreshness } from "@/api/generated/models";

interface LiveStatusProps {
	connection: "connecting" | "connected" | "disconnected";
	freshness: LiveCandleServerMessageFreshness;
	error?: string;
}

export function LiveStatus({ connection, freshness, error }: LiveStatusProps) {
	if (!error && connection === "connected" && freshness === "fresh")
		return null;
	const label =
		error ??
		(connection === "disconnected"
			? "Backend connection lost — showing the last live candle"
			: freshness === "stale"
				? "Binance stream interrupted — showing stale live data"
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
