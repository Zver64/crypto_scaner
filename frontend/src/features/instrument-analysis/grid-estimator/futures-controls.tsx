import { SegmentedControl } from "@mantine/core";

import { FUTURES_GRID_MAX_LEVERAGE } from "@/features/instrument-analysis/grid-estimator/calculator/futures-grid";
import { DIRECTION_OPTIONS } from "@/features/instrument-analysis/grid-estimator/config";
import { marketField } from "@/features/instrument-analysis/grid-estimator/fields";
import type { GridMarket } from "@/features/instrument-analysis/grid-estimator/types";
import type { useGridCalculatorForm } from "@/features/instrument-analysis/grid-estimator/use-grid-calculator-form";

const LEVERAGE_SCALE_VALUES = [1, 5, 10, FUTURES_GRID_MAX_LEVERAGE];

interface FuturesControlsProps {
	disabled: boolean;
	form: ReturnType<typeof useGridCalculatorForm>;
	market: GridMarket;
}

// The position direction and leverage of a futures grid.
export function FuturesControls({
	disabled,
	form,
	market,
}: FuturesControlsProps) {
	return (
		<>
			<form.Field name={marketField(market, "direction")}>
				{(field) => (
					<SegmentedControl
						aria-label="Position direction"
						data={DIRECTION_OPTIONS}
						disabled={disabled}
						fullWidth
						onChange={field.handleChange}
						value={field.state.value}
					/>
				)}
			</form.Field>
			<form.AppField name={marketField(market, "leverage")}>
				{(field) => (
					<field.ScaleSliderField
						disabled={disabled}
						formatValue={(value) => `${value}×`}
						label="Leverage"
						max={FUTURES_GRID_MAX_LEVERAGE}
						min={1}
						scaleLabels={LEVERAGE_SCALE_VALUES.map((value) => ({
							label: `${value}×`,
							position: ((value - 1) / (FUTURES_GRID_MAX_LEVERAGE - 1)) * 100,
						}))}
						step={1}
					/>
				)}
			</form.AppField>
		</>
	);
}
