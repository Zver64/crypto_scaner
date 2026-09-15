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
	secondaryValue?: string;
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
						<SimpleGrid cols={row.items.length} spacing="xs">
							{row.items.map((item) => (
								<Stack gap={2} key={item.label} miw={0}>
									<Text
										c={item.color ?? "dimmed"}
										fw={500}
										size="sm"
										style={{ whiteSpace: "nowrap" }}
									>
										{item.label}
									</Text>
									<Group gap="xs" style={{ rowGap: 0 }} wrap="wrap">
										<Text c={item.color} fw={700}>
											{item.value}
										</Text>
										{item.secondaryValue ? (
											<>
												<Text c={item.color} fw={700} visibleFrom="xs">
													·
												</Text>
												<Text
													c={item.color}
													fw={700}
													w={{ base: "100%", xs: "auto" }}
												>
													{item.secondaryValue}
												</Text>
											</>
										) : null}
									</Group>
								</Stack>
							))}
						</SimpleGrid>
					</Stack>
				))}
			</Stack>
		</Paper>
	);
}
