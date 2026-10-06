import { AppShell, Group, Text } from "@mantine/core";
import { Outlet, useMatches } from "@tanstack/react-router";
import { useGetReadiness } from "@/api/generated/api";
import { AppName } from "@/app/app-name";
import { BusinessRequestContext } from "@/app/business-request-context";
import { OpenInTelegram } from "@/app/open-in-telegram";
import { headerContentHeight } from "@/app/page-height";
import { ReadinessBadge } from "@/app/readiness-badge";
import { useTelegramMiniApp } from "@/app/telegram";
import type { ReadinessStatus } from "@/app/types";
import { useAdministrator } from "@/app/use-administrator";
import { FavoritesProvider } from "@/features/favorites/favorites-provider";
import { getAppVersion } from "@/utils/app-version";
import { getBusinessRequestPermission } from "@/utils/business-request-permission";

export function MiniAppShell() {
	const { webApp } = useTelegramMiniApp();
	const appVersion = getAppVersion(import.meta.env.VITE_APP_VERSION);
	// Retries ride out a slow first check. A later failed check only changes the
	// badge: the backend has been ready once (data is kept on refetch errors),
	// so business requests stay allowed and screens stay mounted.
	const readiness = useGetReadiness({
		query: { refetchInterval: 30_000, retry: 3 },
	});
	const backendReady = readiness.data !== undefined;
	const pageTitle = useMatches({
		select: (matches) => matches.at(-1)?.context.pageTitle,
	});
	const permission = getBusinessRequestPermission({
		backendReady,
		isProduction: import.meta.env.PROD,
		telegramInitData: webApp?.initData,
	});
	const administrator = useAdministrator(permission.allowed);
	const readinessStatus: ReadinessStatus = readiness.isPending
		? "checking"
		: readiness.isError
			? "unavailable"
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
							<AppName administrator={administrator} />
							<Text c="dimmed" size="xs">
								{appVersion}
							</Text>
						</Group>
						{pageTitle ? (
							<Text fw={700} truncate>
								{pageTitle}
							</Text>
						) : null}
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
