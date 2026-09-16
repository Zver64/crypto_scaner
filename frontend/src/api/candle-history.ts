import { useInfiniteQuery } from "@tanstack/react-query";
import { ApiError, parseBackendError } from "@/api/client";
import { getTelegramInitData } from "@/app/telegram";

export const candleIntervals = ["1h", "1d", "1w", "1M"] as const;
export type CandleInterval = (typeof candleIntervals)[number];

export interface PriceCandle {
	close: number;
	high: number;
	low: number;
	open: number;
	open_time: string;
}

export interface CandleRecord extends PriceCandle {
	close_time: string;
	quote_asset_volume: number;
	trade_count: number;
	volume: number;
}

export interface CandlePage {
	candles: CandleRecord[];
	has_more: boolean;
	interval: CandleInterval;
	next_before?: string;
	symbol: string;
}

interface FetchCandlePageOptions {
	before?: string | null;
	initData?: string;
	limit?: number;
	request?: typeof fetch;
}

export async function fetchCandlePage(
	symbol: string,
	interval: CandleInterval,
	options: FetchCandlePageOptions = {},
): Promise<CandlePage> {
	const parameters = new URLSearchParams({ interval });
	if (options.before) parameters.set("before", options.before);
	if (options.limit !== undefined)
		parameters.set("limit", String(options.limit));
	const headers: Record<string, string> = { Accept: "application/json" };
	const initData = options.initData?.trim();
	if (initData) headers.Authorization = `tma ${initData}`;
	try {
		const response = await (options.request ?? fetch)(
			`/api/v1/instruments/${encodeURIComponent(symbol)}/candles?${parameters}`,
			{ headers },
		);
		const payload: unknown = await response.json();
		if (!response.ok) {
			throw parseBackendError(payload, response.status);
		}
		return parseCandlePage(payload, symbol, interval);
	} catch (error) {
		if (error instanceof ApiError) throw error;
		if (error instanceof TypeError) throw new ApiError("network_error");
		throw new ApiError("unexpected_error");
	}
}

export function candleHistoryQueryKey(
	symbol: string,
	interval: CandleInterval,
) {
	return ["candle-history", symbol, interval] as const;
}

export function useCandleHistoryQuery(
	symbol: string,
	interval: CandleInterval,
	enabled: boolean,
) {
	return useInfiniteQuery({
		queryKey: candleHistoryQueryKey(symbol, interval),
		queryFn: ({ pageParam }) =>
			fetchCandlePage(symbol, interval, {
				before: pageParam,
				initData: getTelegramInitData(),
				limit: 200,
			}),
		initialPageParam: null as string | null,
		getNextPageParam: (lastPage) =>
			lastPage.has_more ? lastPage.next_before : undefined,
		enabled,
		retry: false,
		staleTime: Number.POSITIVE_INFINITY,
	});
}

function parseCandlePage(
	payload: unknown,
	expectedSymbol: string,
	expectedInterval: CandleInterval,
): CandlePage {
	if (
		!isRecord(payload) ||
		payload.symbol !== expectedSymbol.toUpperCase() ||
		payload.interval !== expectedInterval ||
		typeof payload.has_more !== "boolean" ||
		!Array.isArray(payload.candles)
	) {
		throw new ApiError("unexpected_error");
	}
	const nextBefore = parseOptionalDate(payload.next_before);
	if (
		(payload.has_more && nextBefore === undefined) ||
		(!payload.has_more && nextBefore !== undefined)
	) {
		throw new ApiError("unexpected_error");
	}
	const candles = payload.candles.map(parseCandle);
	for (let index = 1; index < candles.length; index++) {
		if (
			Date.parse(candles[index - 1].open_time) >=
			Date.parse(candles[index].open_time)
		) {
			throw new ApiError("unexpected_error");
		}
	}
	if (
		nextBefore !== undefined &&
		(candles.length === 0 || nextBefore !== candles[0].open_time)
	) {
		throw new ApiError("unexpected_error");
	}
	return {
		candles,
		has_more: payload.has_more,
		interval: expectedInterval,
		...(nextBefore === undefined ? {} : { next_before: nextBefore }),
		symbol: payload.symbol,
	};
}

function parseCandle(value: unknown): CandleRecord {
	if (!isRecord(value)) throw new ApiError("unexpected_error");
	const openTime = parseDate(value.open_time);
	const closeTime = parseDate(value.close_time);
	const numbers = [
		value.open,
		value.high,
		value.low,
		value.close,
		value.volume,
		value.quote_asset_volume,
		value.trade_count,
	];
	if (
		numbers.some(
			(number) => typeof number !== "number" || !Number.isFinite(number),
		) ||
		Date.parse(closeTime) <= Date.parse(openTime) ||
		(value.high as number) <
			Math.max(value.open as number, value.close as number) ||
		(value.low as number) >
			Math.min(value.open as number, value.close as number) ||
		!Number.isInteger(value.trade_count) ||
		(value.trade_count as number) < 0
	) {
		throw new ApiError("unexpected_error");
	}
	return {
		close: value.close as number,
		close_time: closeTime,
		high: value.high as number,
		low: value.low as number,
		open: value.open as number,
		open_time: openTime,
		quote_asset_volume: value.quote_asset_volume as number,
		trade_count: value.trade_count as number,
		volume: value.volume as number,
	};
}

function parseOptionalDate(value: unknown): string | undefined {
	return value === undefined || value === null ? undefined : parseDate(value);
}

function parseDate(value: unknown): string {
	if (typeof value !== "string" || !Number.isFinite(Date.parse(value))) {
		throw new ApiError("unexpected_error");
	}
	return new Date(value).toISOString();
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}
