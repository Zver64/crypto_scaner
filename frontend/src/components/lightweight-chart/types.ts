import type {
	DeepPartial,
	IChartApi,
	LogicalRange,
	TimeChartOptions,
} from "lightweight-charts";
import type { CSSProperties, ReactNode } from "react";

export interface LightweightChartHandle {
	api(): IChartApi;
	containerWidth(): number;
}

export interface LightweightChartProps {
	"aria-label"?: string;
	children: ReactNode;
	className?: string;
	onVisibleLogicalRangeChange?(range: LogicalRange | null): void;
	options?: DeepPartial<TimeChartOptions>;
	paneStretchFactors?: readonly number[];
	role?: string;
	style?: CSSProperties;
}
