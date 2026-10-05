import { Center, Paper, Stack, Tabs, Title } from "@mantine/core";
import {
	GridEstimator,
	type GridEstimatorProps,
} from "@/features/instrument-analysis/grid-estimator/grid-estimator";

interface GridCalculatorProps extends GridEstimatorProps {
	paperPadding: string;
}

// Spot and USDT-M grid calculators in tabs. Inactive panels stay mounted, so
// each calculator keeps its inputs across tab switches.
export function GridCalculator({
	paperPadding,
	...estimatorProps
}: GridCalculatorProps) {
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
				<Tabs defaultValue="spot">
					<Tabs.List grow>
						<Tabs.Tab value="spot">Spot</Tabs.Tab>
						<Tabs.Tab value="usdm">USDT-M</Tabs.Tab>
					</Tabs.List>
					<Tabs.Panel pt="md" value="spot">
						<GridEstimator {...estimatorProps} market="spot" />
					</Tabs.Panel>
					<Tabs.Panel pt="md" value="usdm">
						<GridEstimator {...estimatorProps} market="usdm" />
					</Tabs.Panel>
				</Tabs>
			</Stack>
		</Paper>
	);
}
