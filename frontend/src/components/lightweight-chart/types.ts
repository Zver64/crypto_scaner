import type { CSSProperties, ReactNode } from "react";

export interface ChartLogicalRange {
	from: number;
	to: number;
}

export interface ChartCanvasHandle {
	generation(): number;
	containerWidth(): number;
	getVisibleLogicalRange(): ChartLogicalRange | null;
	setVisibleLogicalRange(range: ChartLogicalRange): void;
}

export interface ChartCanvasOptions {
	background: string;
	text: string;
	grid: string;
	barSpacing: number;
	minBarSpacing: number;
	timeVisible: boolean;
	timeFormatter(time: number): string;
}

export interface ChartCanvasProps {
	"aria-label"?: string;
	children: ReactNode;
	className?: string;
	onVisibleLogicalRangeChange?(range: ChartLogicalRange | null): void;
	options: ChartCanvasOptions;
	paneStretchFactors?: readonly number[];
	role?: string;
	style?: CSSProperties;
}

export interface ChartCandlestick {
	time: number;
	open: number;
	high: number;
	low: number;
	close: number;
}
export type ChartCandleSlot = ChartCandlestick | { time: number };
export type ChartIndicatorSlot =
	| { time: number; value: number }
	| { time: number };

export interface ChartCandlestickOptions {
	upColor: string;
	downColor: string;
	formatPrice(value: number): string;
	base: number;
	minMove: number;
}
export interface ChartLineOptions {
	color: string;
	bounds: { min: number; max: number };
	formatValue(value: number): string;
	minMove: number;
}
export interface ChartPriceLineOptions {
	lineVisible?: boolean;
	color: string;
	price: number;
	title: string;
}
