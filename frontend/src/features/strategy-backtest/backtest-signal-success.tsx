import { useMantineTheme } from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import type { BacktestSignalOccurrence } from "@/api/generated/models";

interface BacktestSignalSuccessProps {
	outcome: BacktestSignalOccurrence["outcome"];
}

// Show the evaluated outcome; empty when history cannot judge it.
export function BacktestSignalSuccess({ outcome }: BacktestSignalSuccessProps) {
	const { colors } = themeToVars(useMantineTheme());
	if (outcome === null) {
		return null;
	}
	return (
		<span
			style={{
				color:
					outcome === "neutral"
						? undefined
						: outcome === "success"
							? colors.green[6]
							: colors.red[6],
			}}
		>
			{outcome === "neutral" ? "Neutral" : outcome === "success" ? "Yes" : "No"}
		</span>
	);
}
