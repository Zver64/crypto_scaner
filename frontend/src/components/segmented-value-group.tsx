import {
	Box,
	Group,
	Paper,
	SimpleGrid,
	Stack,
	Text,
	Title,
} from "@mantine/core";

interface SegmentedValueGroupItem {
	color?: string;
	label: string;
	value: string;
}

interface SegmentedValueGroupRow {
	ariaLabel: string;
	items: readonly SegmentedValueGroupItem[];
	key: string;
	label?: string;
	segments: readonly {
		color: string;
		key: string;
		percentage: number;
	}[];
	summary: string;
}

interface SegmentedValueGroupProps {
	rows: readonly SegmentedValueGroupRow[];
	title: string;
}

export function SegmentedValueGroup({ rows, title }: SegmentedValueGroupProps) {
	return (
		<Paper withBorder radius="md" p="md" role="group" aria-label={title}>
			<Stack gap="md">
				<Title order={3} size="md">
					{title}
				</Title>
				{rows.map((row) => (
					<Stack gap="sm" key={row.key}>
						<Group
							justify={row.label ? "space-between" : "flex-end"}
							wrap="wrap"
						>
							{row.label ? (
								<Text fw={600} size="sm">
									{row.label}
								</Text>
							) : null}
							<Text c="dimmed" size="sm">
								{row.summary}
							</Text>
						</Group>
						<Box
							aria-label={row.ariaLabel}
							bg="var(--mantine-color-default-hover)"
							h={14}
							role="img"
							style={{ display: "flex", overflow: "hidden" }}
							w="100%"
						>
							{row.segments.map((segment) => (
								<Box
									bg={segment.color}
									key={segment.key}
									style={{ width: `${segment.percentage}%` }}
								/>
							))}
						</Box>
						<SimpleGrid cols={{ base: 1, xs: row.items.length }} spacing="xs">
							{row.items.map((item) => (
								<Stack gap={2} key={item.label}>
									<Text c={item.color ?? "dimmed"} fw={500} size="sm">
										{item.label}
									</Text>
									<Text c={item.color} fw={700}>
										{item.value}
									</Text>
								</Stack>
							))}
						</SimpleGrid>
					</Stack>
				))}
			</Stack>
		</Paper>
	);
}
