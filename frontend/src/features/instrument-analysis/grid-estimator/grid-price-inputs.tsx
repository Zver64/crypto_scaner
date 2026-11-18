import { SimpleGrid } from "@mantine/core";

import type { GridInput } from "@/features/instrument-analysis/grid-estimator/calculator/types";
import type { GridCountRange } from "@/features/instrument-analysis/grid-estimator/estimate";
import { marketField } from "@/features/instrument-analysis/grid-estimator/fields";
import type {
	GridInputField,
	GridMarket,
} from "@/features/instrument-analysis/grid-estimator/types";
import type { useGridCalculatorForm } from "@/features/instrument-analysis/grid-estimator/use-grid-calculator-form";
import type { useGridMarket } from "@/features/instrument-analysis/grid-estimator/use-grid-market";

interface GridPriceInputsProps extends Pick<
	ReturnType<typeof useGridMarket>,
	"commitError" | "commitField"
> {
	committed: GridInput;
	disabled: boolean;
	form: ReturnType<typeof useGridCalculatorForm>;
	// The grid counts Binance accepts.
	gridCounts: GridCountRange | null;
	investmentAsset: string;
	market: GridMarket;
	// The smallest investment Binance accepts, with its currency.
	minimumInvestment: string | null;
	priceUnit: string;
}

// The typed prices, grid count, and investment of a grid calculator. Each is
// checked and committed on blur or Enter, and restored on Escape.
export function GridPriceInputs({
	commitError,
	commitField,
	committed,
	disabled,
	form,
	gridCounts,
	investmentAsset,
	market,
	minimumInvestment,
	priceUnit,
}: GridPriceInputsProps) {
	const inputs: {
		allowDecimal: boolean;
		label: string;
		name: GridInputField;
		placeholder?: string;
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
		{
			allowDecimal: false,
			label: "Grid count",
			name: "gridCount",
			placeholder: gridCounts
				? `${gridCounts.min}–${gridCounts.max}`
				: undefined,
		},
		{
			allowDecimal: true,
			label: `${investmentAsset} investment`,
			name: "investment",
			placeholder: minimumInvestment ? `≥ ${minimumInvestment}` : undefined,
		},
	];
	return (
		<SimpleGrid cols={2} spacing="md">
			{inputs.map(({ allowDecimal, label, name, placeholder }) => (
				<form.AppField
					key={name}
					listeners={{
						onBlur: ({ fieldApi }) => {
							if (fieldApi.state.meta.errors.length === 0) commitField(name);
						},
					}}
					name={marketField(market, name)}
					validators={{ onBlur: ({ value }) => commitError(name, value) }}
				>
					{(field) => (
						<field.NumberField
							allowDecimal={allowDecimal}
							asString
							disabled={disabled}
							label={label}
							placeholder={placeholder}
							onKeyDown={(event) => {
								if (event.key === "Enter") {
									event.preventDefault();
									field.handleBlur();
								}
								if (event.key === "Escape") {
									event.preventDefault();
									field.handleChange(committed[name]);
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
