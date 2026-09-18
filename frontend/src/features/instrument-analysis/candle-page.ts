import type { listInstrumentCandlesResponseSuccess } from "@/api/generated/api";
import type {
	Candle,
	CandleInterval,
	CandlePageResponse,
} from "@/api/generated/models";
import { unexpectedApiError } from "@/features/analysis/api-error";

export type PriceCandle = Pick<
	Candle,
	"close" | "high" | "low" | "open" | "open_time"
>;

export function nextCandlePageParam(
	lastPage: listInstrumentCandlesResponseSuccess,
): string | undefined {
	return lastPage.data.has_more
		? (lastPage.data.next_before ?? undefined)
		: undefined;
}

export function validateCandlePage(
	response: listInstrumentCandlesResponseSuccess,
	expectedSymbol: string,
	expectedInterval: CandleInterval,
): CandlePageResponse {
	const page = response.data;
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
			page.next_before !== page.candles[0].open_time)
	) {
		throw unexpectedApiError();
	}
	return page;
}

export function validateCandles(candles: readonly Candle[]): void {
	for (let index = 0; index < candles.length; index++) {
		const candle = candles[index];
		const openTime = Date.parse(candle.open_time);
		const closeTime = Date.parse(candle.close_time);
		if (
			!Number.isFinite(openTime) ||
			!Number.isFinite(closeTime) ||
			closeTime <= openTime ||
			![
				candle.open,
				candle.high,
				candle.low,
				candle.close,
				candle.volume,
				candle.quote_asset_volume,
			].every(Number.isFinite) ||
			candle.high < Math.max(candle.open, candle.close) ||
			candle.low > Math.min(candle.open, candle.close) ||
			!Number.isInteger(candle.trade_count) ||
			candle.trade_count < 0 ||
			(index > 0 && Date.parse(candles[index - 1].open_time) >= openTime)
		) {
			throw unexpectedApiError();
		}
	}
}
