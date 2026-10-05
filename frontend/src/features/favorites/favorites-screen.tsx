import { Container, Stack } from "@mantine/core";
import { useState } from "react";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { PageNavigation } from "@/app/page-navigation";
import { SettingsForm } from "@/components/settings-form";
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
		<Container maw={880} px={0} size="md">
			<Stack gap="md">
				<PageNavigation current="favorites" title="Favorites" />
				<SettingsForm {...settingsForm} />
				<FavoritesTable
					analysis={analysis}
					onSortChange={onSortChange}
					onSymbolFilterChange={onSymbolFilterChange}
					sort={sort}
					symbolFilter={symbolFilter}
				/>
			</Stack>
		</Container>
	);
}
