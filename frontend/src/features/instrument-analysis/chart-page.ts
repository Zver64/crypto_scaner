import type {
	CandleInterval,
	ChartIndicatorDefinition,
	ChartIndicatorResult,
	ChartPageResponse,
} from "@/api/generated/models";
import { unexpectedApiError } from "@/features/analysis/api-error";
import { validateCandles } from "@/features/instrument-analysis/candle-page";
import { canonicalParameters } from "@/utils/canonical-parameters";

export function validateChartPage(
	page: ChartPageResponse,
	expectedSymbol: string,
	expectedInterval: CandleInterval,
	catalog: readonly ChartIndicatorDefinition[],
): ChartPageResponse {
	if (
		page.symbol !== expectedSymbol.toUpperCase() ||
		page.interval !== expectedInterval ||
		(page.has_more && !page.next_before) ||
		(!page.has_more && page.next_before) ||
		page.indicators.length !== catalog.length
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
	const candleTimes = new Set(page.candles.map((candle) => candle.open_time));
	catalog.forEach((definition, index) => {
		validateIndicatorResult(page.indicators[index], definition, candleTimes);
	});
	return page;
}

// Results arrive in the order the catalog selections were requested. Every
// drawn output must be present; outputs the catalog does not draw are ignored.
// Scale bounds only shape the pane and are not a data constraint.
function validateIndicatorResult(
	result: ChartIndicatorResult | undefined,
	definition: ChartIndicatorDefinition,
	candleTimes: ReadonlySet<string>,
) {
	if (
		result?.type !== definition.type ||
		canonicalParameters(result.parameters) !==
			canonicalParameters(definition.parameters) ||
		definition.lines.some(
			({ output }) => !result.series.some(({ name }) => name === output),
		)
	) {
		throw unexpectedApiError();
	}
	for (const series of result.series) {
		let previous = Number.NEGATIVE_INFINITY;
		for (const point of series.points) {
			const timestamp = Date.parse(point.time);
			if (
				!Number.isFinite(timestamp) ||
				timestamp <= previous ||
				!Number.isFinite(point.value) ||
				!candleTimes.has(point.time)
			) {
				throw unexpectedApiError();
			}
			previous = timestamp;
		}
	}
}

// Replaces the current candle and its indicator points; closed points stay.
export function mergeChartTail(
	base: ChartPageResponse,
	tail: ChartPageResponse,
): ChartPageResponse {
	const from = tail.candles[0]?.open_time;
	if (!from || tail.indicators.length !== base.indicators.length)
		throw unexpectedApiError();
	const since = Date.parse(from);
	const closed = <T>(items: readonly T[], time: (item: T) => string) =>
		items.filter((item) => Date.parse(time(item)) < since);
	const candles = closed(base.candles, (candle) => candle.open_time);
	if (candles.length < base.candles.length - 1) throw unexpectedApiError();
	return {
		...base,
		candles: [...candles, ...tail.candles],
		indicators: base.indicators.map((result, index) => {
			const update = tail.indicators[index];
			if (
				update?.type !== result.type ||
				update.series.length !== result.series.length
			)
				throw unexpectedApiError();
			return {
				...result,
				series: result.series.map((series, seriesIndex) => {
					const points = update.series[seriesIndex];
					if (points?.name !== series.name) throw unexpectedApiError();
					return {
						...series,
						points: [
							...closed(series.points, (point) => point.time),
							...points.points,
						],
					};
				}),
			};
		}),
	};
}
