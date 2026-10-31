import { SliderField } from "@/components/slider-field";
import { AlignBoundButton } from "@/features/instrument-analysis/grid-estimator/align-bound-button";
import type { GridBounds } from "@/features/instrument-analysis/grid-estimator/types";
import { markupScaleLabels } from "@/features/instrument-analysis/grid-estimator/utils";
import { formatRangePercent } from "@/utils/range-percent";

interface MarkupSlidersProps {
	bounds: GridBounds;
	disabled: boolean;
	lowerMarkup: number;
	// Move a price to where the grid is even; null when it already is.
	onLowerAlign: (() => void) | null;
	onLowerMarkupChange: (markup: number) => void;
	onUpperAlign: (() => void) | null;
	onUpperMarkupChange: (markup: number) => void;
	upperMarkup: number;
}

// How far the upper and lower prices sit from the anchor price.
export function MarkupSliders({
	bounds,
	disabled,
	lowerMarkup,
	onLowerAlign,
	onLowerMarkupChange,
	onUpperAlign,
	onUpperMarkupChange,
	upperMarkup,
}: MarkupSlidersProps) {
	return (
		<>
			<SliderField
				disabled={disabled}
				formatValue={formatRangePercent}
				label="Upper price markup"
				labelAction={
					<AlignBoundButton onAlign={disabled ? null : onUpperAlign} />
				}
				max={bounds.upperMarkupMax}
				min={0}
				onChange={onUpperMarkupChange}
				precision={2}
				scaleLabels={markupScaleLabels(bounds.upperMarkupMax)}
				step={0.01}
				value={upperMarkup}
			/>
			<SliderField
				disabled={disabled}
				formatValue={formatRangePercent}
				label="Lower price markup"
				labelAction={
					<AlignBoundButton onAlign={disabled ? null : onLowerAlign} />
				}
				max={bounds.lowerMarkupMax}
				min={0}
				onChange={onLowerMarkupChange}
				precision={2}
				scaleLabels={markupScaleLabels(bounds.lowerMarkupMax)}
				step={0.01}
				value={lowerMarkup}
			/>
		</>
	);
}
