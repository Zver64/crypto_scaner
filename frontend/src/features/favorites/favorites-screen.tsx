import { Stack } from "@mantine/core";
import { useState } from "react";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { PageNavigation } from "@/app/page-navigation";
import { PageStack } from "@/components/page-stack";
import { SettingsForm } from "@/components/settings-form";
import { SidebarLayout } from "@/components/sidebar-layout";
import { sidebarColumns } from "@/components/sidebar-layout/constants";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";
import type { VolatilitySettings } from "@/features/analysis/volatility-settings-form/types";
import { useVolatilitySettingsForm } from "@/features/analysis/volatility-settings-form/use-volatility-settings-form";
import { FavoritesTable } from "@/features/favorites/favorites-table";
import { useFavoritesAnalysis } from "@/features/favorites/use-favorites-analysis";
import type { MarketScanSort } from "@/features/market-scan/sort";

export function FavoritesScreen({
	initialSettings,
	onSettingsCommit,
	onSortChange,
	onSymbolFilterChange,
	sort,
	symbolFilter,
}: {
	initialSettings: VolatilitySettings;
	onSettingsCommit(settings: VolatilitySettings): void;
	onSortChange(sort: MarketScanSort): void;
	onSymbolFilterChange(symbolFilter: string): void;
	sort: MarketScanSort | undefined;
	symbolFilter: string;
}) {
	const permission = useBusinessRequestPermission();
	const wide = useWideLayout();
	const [settings, setSettings] = useState(initialSettings);
	const analysis = useFavoritesAnalysis(settings);
	const settingsForm = useVolatilitySettingsForm({
		disabled: !permission.allowed,
		initialSettings,
		// The previous table stays visible while new settings are analyzed.
		loading: analysis.query.isPlaceholderData,
		onCommit: (nextSettings) => {
			setSettings(nextSettings);
			onSettingsCommit(nextSettings);
		},
	});
	return (
		<PageStack gap="md">
			<PageNavigation current="favorites" title="Favorites" />
			<SidebarLayout
				gap="md"
				sidebar={<SettingsForm {...settingsForm} columns={sidebarColumns} />}
				sidebarPosition="start"
			>
				<Stack flex={wide ? 1 : undefined} gap="md">
					<FavoritesTable
						analysis={analysis}
						fillHeight={wide}
						onSortChange={onSortChange}
						onSymbolFilterChange={onSymbolFilterChange}
						sort={sort}
						symbolFilter={symbolFilter}
					/>
				</Stack>
			</SidebarLayout>
		</PageStack>
	);
}
