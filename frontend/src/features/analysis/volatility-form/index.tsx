import { NumberInputFieldset } from "@/components/number-input-fieldset";
import type { VolatilityFieldsetProps } from "@/features/analysis/volatility-form/types";
import { buildVolatilityInputs } from "@/features/analysis/volatility-form/utils";

export function VolatilityFieldset({
	title,
	...fields
}: VolatilityFieldsetProps) {
	return (
		<NumberInputFieldset inputs={buildVolatilityInputs(fields)} title={title} />
	);
}
