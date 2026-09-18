import type { getInstrumentChartResponseSuccess } from "@/api/generated/api";
import type {
	CandleInterval,
	ChartPageResponse,
	ChartRequest,
	IndicatorPoint,
} from "@/api/generated/models";
import { unexpectedApiError } from "@/features/analysis/api-error";
import { validateCandles } from "@/features/instrument-analysis/candle-page";

export const rsiChartRequest: ChartRequest = {
	indicators: [{ parameters: { period: 14 }, type: "rsi" }],
};

export function nextChartPageParam(
	lastPage: getInstrumentChartResponseSuccess,
): string | undefined {
	return lastPage.data.has_more
		? (lastPage.data.next_before ?? undefined)
		: undefined;
}

export function validateChartPage(
	response: getInstrumentChartResponseSuccess,
	expectedSymbol: string,
	expectedInterval: CandleInterval,
): ChartPageResponse {
	const page = response.data;
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
