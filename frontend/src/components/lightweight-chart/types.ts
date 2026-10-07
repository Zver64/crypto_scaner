import type { DeepPartial, IRange, TimeChartOptions } from "lightweight-charts";
import type { CSSProperties, ReactNode } from "react";

export interface ChartCanvasHandle {
	generation(): number;
	containerWidth(): number;
	fitContent(): void;
	getVisibleLogicalRange(): IRange<number> | null;
	setVisibleLogicalRange(range: IRange<number>): void;
}

export interface ChartCanvasProps {
	"aria-label"?: string;
	children: ReactNode;
	className?: string;
	onVisibleLogicalRangeChange?(range: IRange<number> | null): void;
	options: DeepPartial<TimeChartOptions>;
	paneStretchFactors?: readonly number[];
	role?: string;
	style?: CSSProperties;
}
