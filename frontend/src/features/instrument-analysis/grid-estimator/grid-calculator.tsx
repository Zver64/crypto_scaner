import { Center, Paper, Stack, Tabs, Title } from "@mantine/core";
import { useRef, useState } from "react";

import { GRID_MARKET_TABS } from "@/features/instrument-analysis/grid-estimator/config";
import {
	GridEstimator,
	type GridEstimatorProps,
} from "@/features/instrument-analysis/grid-estimator/grid-estimator";
import type { GridMarket } from "@/features/instrument-analysis/grid-estimator/types";

interface GridCalculatorProps extends GridEstimatorProps {
	paperPadding: string;
}

// Spot, USDT-M, and COIN-M grid calculators in tabs. Inactive panels stay mounted
// and keep their effects running (display-none, not Activity), so each
// calculator keeps its inputs and has reported its levels before its tab is
// selected. Reports the price levels of the selected tab's grid.
export function GridCalculator({
	onPriceLevelsChange,
	paperPadding,
	...estimatorProps
}: GridCalculatorProps) {
	const [market, setMarket] = useState<GridMarket>("spot");
	// The latest levels of every market, read only when a tab is selected.
	const levelsRef = useRef<
		Partial<Record<GridMarket, readonly number[] | null>>
	>({});

	const selectMarket = (value: string | null) => {
		const tab = GRID_MARKET_TABS.find((item) => item.value === value);
		if (!tab) return;
		setMarket(tab.value);
		onPriceLevelsChange(levelsRef.current[tab.value] ?? null);
	};
	const changeLevels = (
		changedMarket: GridMarket,
		changed: readonly number[] | null,
	) => {
		levelsRef.current[changedMarket] = changed;
		if (changedMarket === market) onPriceLevelsChange(changed);
	};

	return (
		<Paper
			component="section"
			aria-busy={estimatorProps.disabled || undefined}
			aria-labelledby="grid-calculator-heading"
			p={paperPadding}
		>
			<Stack gap="md">
				<Center>
					<Title id="grid-calculator-heading" order={2} size="h3">
						Calculator
					</Title>
				</Center>
				<Tabs
					keepMountedMode="display-none"
					onChange={selectMarket}
					value={market}
				>
					<Tabs.List grow>
						{GRID_MARKET_TABS.map(({ label, value }) => (
							<Tabs.Tab key={value} value={value}>
								{label}
							</Tabs.Tab>
						))}
					</Tabs.List>
					{GRID_MARKET_TABS.map(({ value }) => (
						<Tabs.Panel key={value} pt="md" value={value}>
							<GridEstimator
								{...estimatorProps}
								market={value}
								onPriceLevelsChange={(changed) => changeLevels(value, changed)}
							/>
						</Tabs.Panel>
					))}
				</Tabs>
			</Stack>
		</Paper>
	);
}
