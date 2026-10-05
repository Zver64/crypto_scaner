import { Box, Group, Text } from "@mantine/core";

interface LegendItemProps {
	color: string;
	label: string;
	value: string;
}

export function LegendItem({ color, label, value }: LegendItemProps) {
	return (
		<Group gap={6} wrap="nowrap">
			<Box bg={color} h={10} w={10} />
			<Text size="xs">
				<Text c="dimmed" component="span" inherit>
					{label}:
				</Text>{" "}
				{value}
			</Text>
		</Group>
	);
}
