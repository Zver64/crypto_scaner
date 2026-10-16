import { SimpleGrid } from "@mantine/core";

import type { GridInputField } from "@/features/instrument-analysis/grid-estimator/types";
import type { useGridEstimatorForm } from "@/features/instrument-analysis/grid-estimator/use-grid-estimator-form";

type GridEstimatorForm = ReturnType<typeof useGridEstimatorForm>;

interface GridPriceInputsProps extends Pick<
	GridEstimatorForm,
	"commitError" | "commitField" | "committedInput" | "form"
> {
	disabled: boolean;
	investmentAsset: string;
	priceUnit: string;
}

// The typed prices, grid count, and investment of a grid calculator. Each is
// checked and committed on blur or Enter, and restored on Escape.
export function GridPriceInputs({
	commitError,
	commitField,
	committedInput,
	disabled,
	form,
	investmentAsset,
	priceUnit,
}: GridPriceInputsProps) {
	const inputs: {
		allowDecimal: boolean;
		label: string;
		name: GridInputField;
	}[] = [
		{
			allowDecimal: true,
			label: `Lower price (${priceUnit})`,
			name: "lowerPrice",
		},
		{
			allowDecimal: true,
			label: `Upper price (${priceUnit})`,
			name: "upperPrice",
		},
		{ allowDecimal: false, label: "Grid count", name: "gridCount" },
		{
			allowDecimal: true,
			label: `${investmentAsset} investment`,
			name: "investment",
		},
	];
	return (
		<SimpleGrid cols={2} spacing="md">
			{inputs.map(({ allowDecimal, label, name }) => (
				<form.AppField
					key={name}
					listeners={{
						onBlur: ({ fieldApi }) => {
							if (fieldApi.state.meta.errors.length === 0) commitField(name);
						},
					}}
					name={name}
					validators={{ onBlur: ({ value }) => commitError(name, value) }}
				>
					{(field) => (
						<field.NumberField
							allowDecimal={allowDecimal}
							asString
							disabled={disabled}
							label={label}
							onKeyDown={(event) => {
								if (event.key === "Enter") {
									event.preventDefault();
									field.handleBlur();
								}
								if (event.key === "Escape") {
									event.preventDefault();
									field.handleChange(committedInput[name]);
									field.handleBlur();
								}
							}}
							required
						/>
					)}
				</form.AppField>
			))}
		</SimpleGrid>
	);
}
