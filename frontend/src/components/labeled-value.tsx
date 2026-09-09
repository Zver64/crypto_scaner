import { Stack, Text } from "@mantine/core";

interface LabeledValueProps {
	label: string;
	value: string;
}

export function LabeledValue({ label, value }: LabeledValueProps) {
	return (
		<Stack gap={2}>
			<Text c="dimmed" fw={500} size="sm">
				{label}
			</Text>
			<Text fw={700}>{value}</Text>
		</Stack>
	);
}
