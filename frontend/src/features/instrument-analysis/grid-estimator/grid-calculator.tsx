import { Center, Paper, Stack, Tabs, Title } from "@mantine/core";

import { GRID_MARKET_TABS } from "@/features/instrument-analysis/grid-estimator/config";
import {
	GridEstimator,
	type GridEstimatorProps,
} from "@/features/instrument-analysis/grid-estimator/grid-estimator";

interface GridCalculatorProps extends Omit<GridEstimatorProps, "market"> {
	paperPadding: string;
}

// Spot, USDT-M, and COIN-M grid calculators in tabs, on the coin page's
// calculator form, which also holds the selected tab. Inactive panels stay
// mounted (display-none, not Activity).
export function GridCalculator({
	paperPadding,
	...estimatorProps
}: GridCalculatorProps) {
	const { form } = estimatorProps;
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
				<form.Field name="market">
					{(field) => (
						<Tabs
							keepMountedMode="display-none"
							onChange={(value) => {
								const tab = GRID_MARKET_TABS.find(
									(item) => item.value === value,
								);
								if (tab) field.handleChange(tab.value);
							}}
							value={field.state.value}
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
									<GridEstimator {...estimatorProps} market={value} />
								</Tabs.Panel>
							))}
						</Tabs>
					)}
				</form.Field>
			</Stack>
		</Paper>
	);
}
