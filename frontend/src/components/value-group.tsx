import { Paper, SimpleGrid, Stack, Title } from "@mantine/core";
import { LabeledValue } from "@/components/labeled-value";

interface ValueGroupProps {
	title: string;
	items: readonly { label: string; value: string }[];
}

export function ValueGroup({ title, items }: ValueGroupProps) {
	return (
		<Paper withBorder radius="md" p="md" role="group" aria-label={title}>
			<Stack gap="sm">
				<Title order={3} size="md">
					{title}
				</Title>
				<SimpleGrid cols={2} spacing="sm">
					{items.map((item) => (
						<LabeledValue key={item.label} {...item} />
					))}
				</SimpleGrid>
			</Stack>
		</Paper>
	);
}
