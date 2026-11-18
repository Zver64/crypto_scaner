import { AlignBoundButton } from "@/features/instrument-analysis/grid-estimator/align-bound-button";
import { markupScaleLabels } from "@/features/instrument-analysis/grid-estimator/bounds";
import { marketField } from "@/features/instrument-analysis/grid-estimator/fields";
import type {
	GridBounds,
	GridMarket,
} from "@/features/instrument-analysis/grid-estimator/types";
import type { useGridCalculatorForm } from "@/features/instrument-analysis/grid-estimator/use-grid-calculator-form";
import { formatRangePercent } from "@/utils/range-percent";

interface MarkupSlidersProps {
	bounds: GridBounds;
	disabled: boolean;
	form: ReturnType<typeof useGridCalculatorForm>;
	market: GridMarket;
	// Move a price to where the grid is even; null when it already is.
	onLowerAlign: (() => void) | null;
	onLowerMarkupChange: (markup: number) => void;
	onUpperAlign: (() => void) | null;
	onUpperMarkupChange: (markup: number) => void;
}

// How far the upper and lower prices sit from the anchor price.
export function MarkupSliders({
	bounds,
	disabled,
	form,
	market,
	onLowerAlign,
	onLowerMarkupChange,
	onUpperAlign,
	onUpperMarkupChange,
}: MarkupSlidersProps) {
	return (
		<>
			<form.AppField
				listeners={{ onChange: ({ value }) => onUpperMarkupChange(value) }}
				name={marketField(market, "markup")}
			>
				{(field) => (
					<field.ScaleSliderField
						disabled={disabled}
						formatValue={formatRangePercent}
						label="Upper price markup"
						labelAction={
							<AlignBoundButton onAlign={disabled ? null : onUpperAlign} />
						}
						max={bounds.upperMarkupMax}
						min={0}
						precision={2}
						scaleLabels={markupScaleLabels(bounds.upperMarkupMax)}
						step={0.01}
					/>
				)}
			</form.AppField>
			<form.AppField
				listeners={{ onChange: ({ value }) => onLowerMarkupChange(value) }}
				name={marketField(market, "lowerMarkup")}
			>
				{(field) => (
					<field.ScaleSliderField
						disabled={disabled}
						formatValue={formatRangePercent}
						label="Lower price markup"
						labelAction={
							<AlignBoundButton onAlign={disabled ? null : onLowerAlign} />
						}
						max={bounds.lowerMarkupMax}
						min={0}
						precision={2}
						scaleLabels={markupScaleLabels(bounds.lowerMarkupMax)}
						step={0.01}
					/>
				)}
			</form.AppField>
		</>
	);
}
