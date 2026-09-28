import { Group, Text, useMantineTheme } from "@mantine/core";
import type { ReactNode } from "react";
import { useCoinPageLayout } from "@/features/instrument-analysis/use-coin-page-layout";

interface OverviewRowProps {
	// Highlighted values use the theme's primary color.
	highlighted?: boolean;
	label: string;
	value: ReactNode;
}

export function OverviewRow({ highlighted, label, value }: OverviewRowProps) {
	const { textSize } = useCoinPageLayout();
	const { colors, fontWeights, primaryColor } = useMantineTheme();
	return (
		<Group justify="space-between" wrap="nowrap">
			<Text size={textSize}>{label}</Text>
			<Text
				c={highlighted ? colors[primaryColor]?.[4] : undefined}
				fw={fontWeights.bold}
				size={textSize}
				ta="right"
			>
				{value}
			</Text>
		</Group>
	);
}
