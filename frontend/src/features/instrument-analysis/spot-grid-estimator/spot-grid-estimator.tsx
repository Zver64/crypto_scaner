import {
	Alert,
	Paper,
	SimpleGrid,
	Stack,
	Text,
	TextInput,
	Title,
} from "@mantine/core";
import { useMemo, useState } from "react";
import {
	calculateSpotGridInput,
	spotGridEstimateValues,
} from "@/features/instrument-analysis/spot-grid-estimator/utils";
import type { ArithmeticSpotGridInput } from "@/utils/calculator/arithmetic-spot-grid";

interface SpotGridEstimatorProps {
	paperPadding: string;
}

const emptyInput: ArithmeticSpotGridInput = {
	lowerPrice: "",
	upperPrice: "",
	gridCount: "",
	investment: "",
};

function EstimateValue({ label, value }: { label: string; value: string }) {
	return (
		<Stack gap={2}>
			<Text fw={500} size="sm">
				{label}
			</Text>
			<Text fw={700}>{value}</Text>
		</Stack>
	);
}

export function SpotGridEstimator({ paperPadding }: SpotGridEstimatorProps) {
	const [input, setInput] = useState(emptyInput);
	const calculation = useMemo(() => calculateSpotGridInput(input), [input]);
	const estimate = calculation?.estimate ?? null;
	const values = spotGridEstimateValues(estimate);

	function update(field: keyof ArithmeticSpotGridInput, value: string) {
		setInput((current) => ({ ...current, [field]: value }));
	}

	return (
		<Paper
			component="section"
			aria-labelledby="spot-grid-estimator-heading"
			p={paperPadding}
		>
			<Stack gap="md">
				<Title id="spot-grid-estimator-heading" order={2} size="h3">
					Spot Grid Calculator
				</Title>
				<SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
					<TextInput
						inputMode="decimal"
						label="Lower price (USDT)"
						onChange={(event) =>
							update("lowerPrice", event.currentTarget.value)
						}
						required
						value={input.lowerPrice}
					/>
					<TextInput
						inputMode="decimal"
						label="Upper price (USDT)"
						onChange={(event) =>
							update("upperPrice", event.currentTarget.value)
						}
						required
						value={input.upperPrice}
					/>
					<TextInput
						inputMode="numeric"
						label="Grid count"
						onChange={(event) => update("gridCount", event.currentTarget.value)}
						required
						value={input.gridCount}
					/>
					<TextInput
						inputMode="decimal"
						label="USDT investment"
						onChange={(event) =>
							update("investment", event.currentTarget.value)
						}
						required
						value={input.investment}
					/>
				</SimpleGrid>

				{calculation?.error ? (
					<Alert color="red" title="Check calculator inputs">
						{calculation.error}
					</Alert>
				) : null}

				<SimpleGrid cols={2} spacing="md" aria-live="polite">
					<EstimateValue
						label="Profit per step"
						value={`${values.profitPerStep}, ${values.profitPerStepPercent}`}
					/>
					<EstimateValue
						label="Average entry price"
						value={values.averageEntryPrice}
					/>
				</SimpleGrid>
			</Stack>
		</Paper>
	);
}
