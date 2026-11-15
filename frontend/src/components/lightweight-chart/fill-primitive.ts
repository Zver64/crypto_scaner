import type {
	IPrimitivePaneRenderer,
	IPrimitivePaneView,
	ISeriesPrimitive,
	SeriesAttachedParameter,
	Time,
} from "lightweight-charts";

import type { SeriesFillPoint } from "@/components/lightweight-chart/types";

export interface FillPrimitive extends ISeriesPrimitive<Time> {
	setPoints(points: readonly SeriesFillPoint[]): void;
}

interface FillPolygon {
	color: string;
	// The outline: along the values, then back along the bases.
	outline: { x: number; y: number }[];
}

// Fills the area between the values and bases of points on the price scale of
// the series it is attached to, behind the series. Neighbouring points of the
// same color are joined; a gap or a change of color breaks the fill.
export function createFillPrimitive(
	initialPoints: readonly SeriesFillPoint[],
): FillPrimitive {
	let points = initialPoints;
	let host: SeriesAttachedParameter<Time> | null = null;
	let polygons: FillPolygon[] = [];

	const renderer: IPrimitivePaneRenderer = {
		draw: (target) =>
			target.useMediaCoordinateSpace(({ context }) => {
				for (const { color, outline } of polygons) {
					context.beginPath();
					for (const { x, y } of outline) context.lineTo(x, y);
					context.closePath();
					context.fillStyle = color;
					context.fill();
				}
			}),
	};
	const view: IPrimitivePaneView = {
		renderer: () => renderer,
		zOrder: () => "bottom",
	};

	return {
		attached(parameter) {
			host = parameter;
			// Attaching does not redraw the chart.
			parameter.requestUpdate();
		},
		detached() {
			host = null;
		},
		paneViews: () => [view],
		updateAllViews() {
			polygons = [];
			// The chart draws the primitives of a hidden series too.
			if (host === null || !host.series.options().visible) return;
			const { chart, series } = host;
			const timeScale = chart.timeScale();
			let values: { x: number; y: number }[] = [];
			let bases: { x: number; y: number }[] = [];
			let color = "";
			const close = () => {
				if (values.length > 1) {
					polygons.push({ color, outline: [...values, ...bases.reverse()] });
				}
				values = [];
				bases = [];
			};
			for (const point of points) {
				const x = timeScale.timeToCoordinate(point.time);
				if (!("value" in point) || x === null) {
					close();
					continue;
				}
				const value = series.priceToCoordinate(point.value);
				const base = series.priceToCoordinate(point.base);
				if (value === null || base === null) {
					close();
					continue;
				}
				if (point.color !== color) close();
				color = point.color;
				values.push({ x, y: value });
				bases.push({ x, y: base });
			}
			close();
		},
		setPoints(next) {
			points = next;
			host?.requestUpdate();
		},
	};
}
