import type { ReactNode } from "react";
import type { AnalysisUnit } from "@/features/analysis/criteria";

export type VolatilityFormValue = number | string;

export interface VolatilityFormField {
	error?: ReactNode;
	id: string;
	onChange(value: VolatilityFormValue): void;
	value: VolatilityFormValue;
}

export interface VolatilityFieldsetProps {
	minimumRangePercent?: VolatilityFormField;
	minimumRangePresets?: readonly number[];
	percentile: VolatilityFormField;
	percentilePresets: readonly number[];
	period: VolatilityFormField;
	periodPresets: readonly number[];
	size?: string;
	title: string;
	unit: AnalysisUnit;
}
