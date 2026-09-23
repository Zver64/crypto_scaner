import {
	AppShell,
	Badge,
	Group,
	Paper,
	Stack,
	Text,
	Title,
} from "@mantine/core";
import { Outlet } from "@tanstack/react-router";
import { useGetReadiness } from "@/api/generated/api";
import { ShellContentCenter } from "@/components/shell-content-center";
import { FavoritesProvider } from "@/features/favorites/favorites-provider";
import { getAppVersion } from "@/utils/app-version";
import { getBusinessRequestPermission } from "@/utils/business-request-permission";
import { BusinessRequestContext } from "./business-request-context";
import { useTelegramMiniApp } from "./telegram";

const headerContentHeight = "3.25rem";

type ReadinessStatus = "checking" | "ready" | "unavailable";

const readinessPresentation = {
	checking: { color: "yellow", label: "Checking" },
	ready: { color: "teal", label: "Ready" },
	unavailable: { color: "red", label: "Unavailable" },
} as const;

export function MiniAppShell() {
	const { webApp } = useTelegramMiniApp();
	const appVersion = getAppVersion(import.meta.env.VITE_APP_VERSION);
	const readiness = useGetReadiness({
		query: {
			refetchInterval: 30_000,
			retry: false,
			select: (response) => response.status === 200,
		},
	});
	const backendReady = readiness.data === true;
	const permission = getBusinessRequestPermission({
		backendReady,
		isProduction: import.meta.env.PROD,
		telegramInitData: webApp?.initData,
	});
	const readinessStatus: ReadinessStatus = readiness.isPending
		? "checking"
		: backendReady
			? "ready"
			: "unavailable";

	return (
		<BusinessRequestContext value={permission}>
			<AppShell
				header={{ height: headerContentHeight }}
				padding={{ base: "xs", sm: "sm" }}
				withBorder={false}
			>
				<AppShell.Header>
					<Group
						h={headerContentHeight}
						justify="space-between"
						px={{ base: "xs", sm: "sm" }}
					>
						<Group gap="xs">
							<Text fw={800} lts="0.08em">
								CS
							</Text>
							<Text c="dimmed" size="xs">
								{appVersion}
							</Text>
						</Group>
						<ReadinessBadge status={readinessStatus} />
					</Group>
				</AppShell.Header>
				<AppShell.Main>
					{permission.authenticated ? (
						<FavoritesProvider enabled={permission.allowed}>
							<Outlet />
						</FavoritesProvider>
					) : (
						<OpenInTelegram />
					)}
				</AppShell.Main>
			</AppShell>
		</BusinessRequestContext>
	);
}

function ReadinessBadge({ status }: { status: ReadinessStatus }) {
	const presentation = readinessPresentation[status];

	return (
		<Badge color={presentation.color} size="sm" variant="light">
			{presentation.label}
		</Badge>
	);
}

function OpenInTelegram() {
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
