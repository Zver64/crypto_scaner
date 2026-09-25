import type {
	CandleInterval,
	ChartPageResponse,
	IndicatorConfig,
	IndicatorPoint,
} from "@/api/generated/models";
import { unexpectedApiError } from "@/features/analysis/api-error";
import { validateCandles } from "@/features/instrument-analysis/candle-page";

export const rsiIndicators: IndicatorConfig[] = [
	{ parameters: { period: 14 }, type: "rsi" },
];

export function validateChartPage(
	page: ChartPageResponse,
	expectedSymbol: string,
	expectedInterval: CandleInterval,
): ChartPageResponse {
	if (
		page.symbol !== expectedSymbol.toUpperCase() ||
		page.interval !== expectedInterval ||
		(page.has_more && !page.next_before) ||
		(!page.has_more && page.next_before) ||
		page.indicators.length !== 1
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
	const result = page.indicators[0];
	const series = result?.series;
	if (
		result?.type !== "rsi" ||
		result.parameters.period !== 14 ||
		series?.length !== 1 ||
		series[0]?.name !== "rsi"
	) {
		throw unexpectedApiError();
	}
	const candleTimes = new Set(page.candles.map((candle) => candle.open_time));
	let previous = Number.NEGATIVE_INFINITY;
	for (const point of series[0].points) {
		const timestamp = Date.parse(point.time);
		if (
			!Number.isFinite(timestamp) ||
			timestamp <= previous ||
			!Number.isFinite(point.value) ||
			point.value < 0 ||
			point.value > 100 ||
			!candleTimes.has(point.time)
		) {
			throw unexpectedApiError();
		}
		previous = timestamp;
	}
	return page;
}

export function rsiPoints(page: ChartPageResponse): readonly IndicatorPoint[] {
	return page.indicators[0]?.series[0]?.points ?? [];
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
