import { Paper, Stack, Text, Title } from "@mantine/core";
import { ShellContentCenter } from "@/components/shell-content-center";

export function OpenInTelegram() {
	return (
		<ShellContentCenter>
			<Paper maw={420} p={{ base: "xs", sm: "xl" }} radius="lg" shadow="sm">
				<Stack align="center" gap="sm" ta="center">
					<Title order={1} size="h2">
						Open in Telegram
					</Title>
					<Text c="dimmed">
						Launch this Mini App from Telegram to continue securely.
					</Text>
				</Stack>
			</Paper>
		</ShellContentCenter>
	);
}
