import { SliderField } from "@/components/slider-field";
import type {
	GridBounds,
	GridMarkups,
} from "@/features/instrument-analysis/grid-estimator/types";
import { markupScaleLabels } from "@/features/instrument-analysis/grid-estimator/utils";
import { formatRangePercent } from "@/utils/range-percent";

interface MarkupSlidersProps {
	bounds: GridBounds;
	defaultMarkups: GridMarkups;
	disabled: boolean;
	lowerMarkup: number;
	onLowerMarkupChange: (markup: number) => void;
	onUpperMarkupChange: (markup: number) => void;
	upperMarkup: number;
}

// How far the upper and lower prices sit from the anchor price.
export function MarkupSliders({
	bounds,
	defaultMarkups,
	disabled,
	lowerMarkup,
	onLowerMarkupChange,
	onUpperMarkupChange,
	upperMarkup,
}: MarkupSlidersProps) {
	return (
		<>
			<SliderField
				disabled={disabled}
				formatValue={formatRangePercent}
				label="Upper price markup"
				max={bounds.upperMarkupMax}
				min={0}
				onChange={onUpperMarkupChange}
				precision={2}
				scaleLabels={markupScaleLabels(
					bounds.upperMarkupMax,
					defaultMarkups.upper,
				)}
				step={0.01}
				value={upperMarkup}
			/>
			<SliderField
				disabled={disabled}
				formatValue={formatRangePercent}
				label="Lower price markup"
				max={bounds.lowerMarkupMax}
				min={0}
				onChange={onLowerMarkupChange}
				precision={2}
				scaleLabels={markupScaleLabels(
					bounds.lowerMarkupMax,
					defaultMarkups.lower,
				)}
				step={0.01}
				value={lowerMarkup}
			/>
		</>
	);
}
