import {
	AppShell,
	Badge,
	Group,
	Paper,
	Stack,
	Text,
	Title,
} from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import { Outlet } from "@tanstack/react-router";
import { BusinessRequestContext } from "./business-request-context";
import { fetchReadiness, readinessQueryKey } from "./readiness";
import { getBusinessRequestPermission } from "./request-permission";
import { ShellContentCenter } from "./shell-content-center";
import { useTelegramMiniApp } from "./telegram";

const headerContentHeight = "3.25rem";

type ReadinessStatus = "checking" | "ready" | "unavailable";

const readinessPresentation = {
	checking: { color: "yellow", label: "Checking" },
	ready: { color: "teal", label: "Ready" },
	unavailable: { color: "red", label: "Unavailable" },
} as const;

export function MiniAppShell() {
	const { safeAreaInsets, webApp } = useTelegramMiniApp();
	const readiness = useQuery({
		queryFn: () => fetchReadiness(),
		queryKey: readinessQueryKey,
		refetchInterval: 30_000,
		retry: false,
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
	const headerHeight = `calc(${headerContentHeight} + max(env(safe-area-inset-top, 0px), ${safeAreaInsets.top}px))`;

	return (
		<BusinessRequestContext value={permission}>
			<AppShell
				header={{ height: headerHeight }}
				padding="sm"
				styles={{
					header: {
						paddingLeft: `max(env(safe-area-inset-left, 0px), ${safeAreaInsets.left}px)`,
						paddingRight: `max(env(safe-area-inset-right, 0px), ${safeAreaInsets.right}px)`,
						paddingTop: `max(env(safe-area-inset-top, 0px), ${safeAreaInsets.top}px)`,
					},
					main: {
						minHeight: "100dvh",
						paddingBottom: `calc(var(--mantine-spacing-sm) + max(env(safe-area-inset-bottom, 0px), ${safeAreaInsets.bottom}px))`,
						paddingLeft: `calc(var(--mantine-spacing-sm) + max(env(safe-area-inset-left, 0px), ${safeAreaInsets.left}px))`,
						paddingRight: `calc(var(--mantine-spacing-sm) + max(env(safe-area-inset-right, 0px), ${safeAreaInsets.right}px))`,
					},
				}}
				withBorder
			>
				<AppShell.Header>
					<Group h={headerContentHeight} justify="space-between" px="sm">
						<Text fw={800} lts="0.08em">
							CS
						</Text>
						<ReadinessBadge status={readinessStatus} />
					</Group>
				</AppShell.Header>
				<AppShell.Main>
					{permission.authenticated ? <Outlet /> : <OpenInTelegram />}
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
			<Paper maw={420} p="xl" radius="lg" shadow="sm" withBorder>
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
