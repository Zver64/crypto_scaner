import { SegmentedControl } from "@mantine/core";
import { SliderField } from "@/components/slider-field";
import { DIRECTION_OPTIONS } from "@/features/instrument-analysis/grid-estimator/config";
import { FUTURES_GRID_MAX_LEVERAGE } from "@/utils/calculator/futures-grid";
import type { PositionDirection } from "@/utils/calculator/types";

interface FuturesControlsProps {
	direction: PositionDirection;
	disabled: boolean;
	leverage: number;
	onDirectionChange: (direction: PositionDirection) => void;
	onLeverageChange: (leverage: number) => void;
}

// The position direction and leverage of a futures grid.
export function FuturesControls({
	direction,
	disabled,
	leverage,
	onDirectionChange,
	onLeverageChange,
}: FuturesControlsProps) {
	return (
		<>
			<SegmentedControl
				aria-label="Position direction"
				data={DIRECTION_OPTIONS}
				disabled={disabled}
				fullWidth
				onChange={onDirectionChange}
				value={direction}
			/>
			<SliderField
				disabled={disabled}
				formatValue={(value) => `${value}×`}
				label="Leverage"
				max={FUTURES_GRID_MAX_LEVERAGE}
				min={1}
				onChange={onLeverageChange}
				scaleLabels={Array.from(
					{ length: FUTURES_GRID_MAX_LEVERAGE },
					(_, index) => ({
						label: `${index + 1}×`,
						position: (index / (FUTURES_GRID_MAX_LEVERAGE - 1)) * 100,
					}),
				)}
				step={1}
				value={leverage}
			/>
		</>
	);
}
