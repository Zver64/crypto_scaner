import { Paper, Stack, Text, Title } from "@mantine/core";
import { createFileRoute } from "@tanstack/react-router";
import { ShellContentCenter } from "../app/shell-content-center";

export const Route = createFileRoute("/")({ component: Home });

function Home() {
	return (
		<ShellContentCenter>
			<Paper maw={480} p="xl" radius="lg" shadow="sm" withBorder>
				<Stack gap="xs">
					<Title order={1} size="h2">
						Crypto Scanner
					</Title>
					<Text c="dimmed">
						The Mini App shell is ready. Market scanning arrives in the next
						delivery.
					</Text>
				</Stack>
			</Paper>
		</ShellContentCenter>
	);
}
