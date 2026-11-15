import type {
	CandleInterval,
	ChartPageResponse,
	ChartShape,
} from "@/api/generated/models";
import { unexpectedApiError } from "@/features/analysis/api-error";
import { validateCandles } from "@/features/instrument-analysis/candle-page";

export function validateChartPage(
	page: ChartPageResponse,
	expectedSymbol: string,
	expectedInterval: CandleInterval,
): ChartPageResponse {
	if (
		page.symbol !== expectedSymbol.toUpperCase() ||
		page.interval !== expectedInterval ||
		(page.has_more && !page.next_before) ||
		(!page.has_more && page.next_before)
	) {
		throw unexpectedApiError();
	}
	validateCandles(page.candles);
	if (
		page.next_before &&
		(!Number.isFinite(Date.parse(page.next_before)) ||
			page.candles.length === 0 ||
			page.next_before !== page.candles[0]?.open_time)
	) {
		throw unexpectedApiError();
	}
	validateShapes(page);
	return page;
}

// Shapes have distinct ids, lie in panes the page has, and lie on its
// candles; line and area points follow each other in time. Scale bounds only
// shape the pane and are not a data constraint.
function validateShapes({ candles, panes, shapes }: ChartPageResponse) {
	const candleTimes = new Set(
		candles.map((candle) => Date.parse(candle.open_time)),
	);
	const paneKeys = new Set(panes.map(({ key }) => key));
	if (
		paneKeys.size !== panes.length ||
		new Set(shapes.map(({ id }) => id)).size !== shapes.length
	) {
		throw unexpectedApiError();
	}
	for (const shape of shapes) {
		const inPane = shape.pane !== undefined && paneKeys.has(shape.pane);
		const times = shapeTimes(shape).map((time) => Date.parse(time));
		if (
			(shape.placement === "pane") !== inPane ||
			(shape.placement !== "pane" && shape.pane !== undefined) ||
			times.some((time) => !candleTimes.has(time))
		) {
			throw unexpectedApiError();
		}
		if (
			(shape.kind === "line" || shape.kind === "area") &&
			times.some((time, index) => index > 0 && time <= (times[index - 1] ?? 0))
		) {
			throw unexpectedApiError();
		}
	}
}

// The candle times a shape lies on.
function shapeTimes(shape: ChartShape): string[] {
	switch (shape.kind) {
		case "line":
		case "area":
			return shape.points.map(({ time }) => time);
		case "markers":
			return shape.points.flatMap(({ calculated_time, time }) => [
				time,
				calculated_time,
			]);
		case "segments":
			return shape.segments.flatMap(({ calculated_time, from, to }) => [
				from.time,
				to.time,
				calculated_time,
			]);
	}
	return [];
}

// Replaces the current candle and, of every shape, what was calculated from
// it on; what closed candles calculated stays.
export function mergeChartTail(
	base: ChartPageResponse,
	tail: ChartPageResponse,
): ChartPageResponse {
	const from = tail.candles[0]?.open_time;
	if (!from || tail.shapes.length !== base.shapes.length)
		throw unexpectedApiError();
	const since = Date.parse(from);
	const closed = <T>(items: readonly T[], time: (item: T) => string) =>
		items.filter((item) => Date.parse(time(item)) < since);
	const candles = closed(base.candles, (candle) => candle.open_time);
	if (candles.length < base.candles.length - 1) throw unexpectedApiError();
	const updates = new Map(tail.shapes.map((shape) => [shape.id, shape]));
	return {
		...base,
		candles: [...candles, ...tail.candles],
		shapes: base.shapes.map((shape): ChartShape => {
			const update = updates.get(shape.id);
			if (shape.kind === "line" && update?.kind === "line") {
				return {
					...shape,
					points: [
						...closed(shape.points, ({ time }) => time),
						...update.points,
					],
				};
			}
			if (shape.kind === "area" && update?.kind === "area") {
				return {
					...shape,
					points: [
						...closed(shape.points, ({ time }) => time),
						...update.points,
					],
				};
			}
			if (shape.kind === "markers" && update?.kind === "markers") {
				return {
					...shape,
					points: [
						...closed(shape.points, (marker) => marker.calculated_time),
						...update.points,
					],
				};
			}
			if (shape.kind === "segments" && update?.kind === "segments") {
				return {
					...shape,
					segments: [
						...closed(shape.segments, (segment) => segment.calculated_time),
						...update.segments,
					],
				};
			}
			throw unexpectedApiError();
		}),
	};
}
