import { SimpleGrid, TextInput } from "@mantine/core";
import type { GridInputField } from "@/features/instrument-analysis/grid-estimator/types";
import type { useGridEstimatorForm } from "@/features/instrument-analysis/grid-estimator/use-grid-estimator-form";

interface GridPriceInputsProps {
	disabled: boolean;
	inputProps: ReturnType<typeof useGridEstimatorForm>["inputProps"];
	investmentAsset: string;
	priceUnit: string;
}

// The typed prices, grid count, and investment of a grid calculator.
export function GridPriceInputs({
	disabled,
	inputProps,
	investmentAsset,
	priceUnit,
}: GridPriceInputsProps) {
	const field = (name: GridInputField) => ({ disabled, ...inputProps(name) });
	return (
		<SimpleGrid cols={2} spacing="md">
			<TextInput
				inputMode="decimal"
				label={`Lower price (${priceUnit})`}
				required
				{...field("lowerPrice")}
			/>
			<TextInput
				inputMode="decimal"
				label={`Upper price (${priceUnit})`}
				required
				{...field("upperPrice")}
			/>
			<TextInput
				inputMode="numeric"
				label="Grid count"
				required
				{...field("gridCount")}
			/>
			<TextInput
				inputMode="decimal"
				label={`${investmentAsset} investment`}
				required
				{...field("investment")}
			/>
		</SimpleGrid>
	);
}
