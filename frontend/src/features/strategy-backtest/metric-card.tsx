import { Paper, Stack, Text } from "@mantine/core";
import type { ReactNode } from "react";
import { HintLabel } from "@/features/strategy-backtest/hint-label";

interface MetricCardProps {
	// A dimmed line under the value, such as a baseline to compare with.
	comparison?: ReactNode;
	hint: string;
	label: string;
	paperPadding: string;
	value: ReactNode;
}

// One headline metric of the strategy with a large value.
export function MetricCard({
	comparison,
	hint,
	label,
	paperPadding,
	value,
}: MetricCardProps) {
	return (
		<Paper p={paperPadding}>
			<Stack gap={2}>
				<Text c="dimmed" size="xs">
					<HintLabel hint={hint}>{label}</HintLabel>
				</Text>
				<Text fw={700} fz="xl">
					{value}
				</Text>
				{comparison ? (
					<Text c="dimmed" size="xs">
						{comparison}
					</Text>
				) : null}
			</Stack>
		</Paper>
	);
}
