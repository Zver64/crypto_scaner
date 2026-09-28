import {
	AppShell,
	Badge,
	Group,
	Paper,
	Stack,
	Text,
	Title,
} from "@mantine/core";
import { Outlet, useMatches } from "@tanstack/react-router";
import { useGetReadiness } from "@/api/generated/api";
import { ShellContentCenter } from "@/components/shell-content-center";
import { FavoritesProvider } from "@/features/favorites/favorites-provider";
import { getAppVersion } from "@/utils/app-version";
import { getBusinessRequestPermission } from "@/utils/business-request-permission";
import { BusinessRequestContext } from "./business-request-context";
import { useTelegramMiniApp } from "./telegram";

const headerContentHeight = "3.25rem";

type ReadinessStatus =
	| "checking"
	| "ready"
	| "auditing"
	| "auditFailed"
	| "unavailable";

const readinessPresentation = {
	checking: { color: "yellow", label: "Checking" },
	ready: { color: "teal", label: "Ready" },
	auditing: { color: "blue", label: "Audit" },
	auditFailed: { color: "orange", label: "Audit failed" },
	unavailable: { color: "red", label: "Unavailable" },
} as const;

// Progress is polled faster while a background security audit runs.
const auditPollInterval = 5_000;
const readinessPollInterval = 30_000;

export function MiniAppShell() {
	const { webApp } = useTelegramMiniApp();
	const appVersion = getAppVersion(import.meta.env.VITE_APP_VERSION);
	const readiness = useGetReadiness({
		query: {
			refetchInterval: (query) =>
				query.state.data?.data.background.token_security.status === "running"
					? auditPollInterval
					: readinessPollInterval,
			retry: false,
			select: (response) => ({
				ready: response.status === 200,
				tokenSecurity: response.data.background.token_security,
			}),
		},
	});
	const backendReady = readiness.data?.ready === true;
	const tokenSecurity = readiness.data?.tokenSecurity;
	const pageTitle = useMatches({
		select: (matches) => matches.at(-1)?.context.pageTitle,
	});
	const permission = getBusinessRequestPermission({
		backendReady,
		isProduction: import.meta.env.PROD,
		telegramInitData: webApp?.initData,
	});
	const readinessStatus: ReadinessStatus = readiness.isPending
		? "checking"
		: !backendReady
			? "unavailable"
			: tokenSecurity?.status === "running"
				? "auditing"
				: tokenSecurity?.status === "failed"
					? "auditFailed"
					: "ready";

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
						{pageTitle ? (
							<Text fw={700} truncate>
								{pageTitle}
							</Text>
						) : null}
						<ReadinessBadge
							progress={
								readinessStatus === "auditing" && tokenSecurity
									? `${tokenSecurity.completed}/${tokenSecurity.total}`
									: undefined
							}
							status={readinessStatus}
						/>
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

function ReadinessBadge({
	progress,
	status,
}: {
	progress?: string;
	status: ReadinessStatus;
}) {
	const presentation = readinessPresentation[status];

	return (
		<Badge color={presentation.color} size="sm" variant="light">
			{progress ? `${presentation.label} ${progress}` : presentation.label}
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
