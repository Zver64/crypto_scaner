import { Center, Loader, Paper, Text } from "@mantine/core";

interface CoinChartPlaceholderProps {
	failed: boolean;
	paperPadding: string;
}

// Holds the chart's place while the indicator catalog loads, or explains why
// the chart cannot be shown.
export function CoinChartPlaceholder({
	failed,
	paperPadding,
}: CoinChartPlaceholderProps) {
	return (
		<Paper component="section" p={paperPadding}>
			<Center mih={200}>
				{failed ? (
					<Text c="dimmed" size="sm">
						The chart is unavailable. Please try again later.
					</Text>
				) : (
					<Loader aria-label="Loading chart" />
				)}
			</Center>
		</Paper>
	);
}
