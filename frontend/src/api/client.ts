import type { InstrumentAnalysisCriteria } from "../features/instrument-analysis/criteria";
import type { MarketScanCriteria } from "../features/market-scan/criteria";

export interface InstrumentAnalysisResult {
	candle_count: number;
	from: string;
	percentile: number;
	period_days: number;
	range_percent: number;
	symbol: string;
	to: string;
}

export interface MarketScanItem {
	candle_count: number;
	range_percent: number;
	symbol: string;
}

export interface MarketScanResult {
	analyzed_count: number;
	insufficient_data_count: number;
	items: MarketScanItem[];
	matched_count: number;
	minimum_range_percent: number;
	percentile: number;
	period_days: number;
}

type ApiErrorCode =
	| "access_denied"
	| "insufficient_data"
	| "invalid_argument"
	| "market_data_unavailable"
	| "network_error"
	| "symbol_not_found"
	| "unauthenticated"
	| "unexpected_error";

const apiErrorMessages: Record<ApiErrorCode, string> = {
	access_denied: "Your Telegram account does not have access to this scanner.",
	insufficient_data: "There is not enough market history for this analysis.",
	invalid_argument: "The request contains unsupported analysis criteria.",
	market_data_unavailable: "Market data is not ready yet. Try again shortly.",
	network_error:
		"Unable to reach the scanner. Check your connection and try again.",
	symbol_not_found: "This instrument is unknown or no longer active.",
	unauthenticated:
		"Your Telegram authorization has expired. Reopen the Mini App and try again.",
	unexpected_error: "An unexpected error occurred. Please try again.",
};

export class ApiError extends Error {
	readonly code: ApiErrorCode;
	readonly requestId?: string;
	readonly status?: number;

	constructor(
		code: ApiErrorCode,
		options: { requestId?: string; status?: number } = {},
	) {
		super(apiErrorMessages[code]);
		this.name = "ApiError";
		this.code = code;
		this.requestId = options.requestId;
		this.status = options.status;
	}
}

interface FetchAnalysisOptions {
	initData?: string;
	request?: typeof fetch;
}

export async function fetchMarketScan(
	criteria: MarketScanCriteria,
	options: FetchAnalysisOptions = {},
): Promise<MarketScanResult> {
	const parameters = new URLSearchParams({
		period_days: String(criteria.periodDays),
		percentile: String(criteria.percentile),
		minimum_range_percent: String(criteria.minimumRangePercent),
	});
	return fetchAnalysisResult(
		`/api/v1/analysis/percentile?${parameters.toString()}`,
		options,
		parseMarketScanResult,
	);
}

export async function fetchInstrumentAnalysis(
	symbol: string,
	criteria: InstrumentAnalysisCriteria,
	options: FetchAnalysisOptions = {},
): Promise<InstrumentAnalysisResult> {
	const parameters = new URLSearchParams({
		period_days: String(criteria.periodDays),
		percentile: String(criteria.percentile),
	});
	return fetchAnalysisResult(
		`/api/v1/analysis/percentile/${encodeURIComponent(symbol)}?${parameters.toString()}`,
		options,
		parseInstrumentAnalysisResult,
	);
}

async function fetchAnalysisResult<Result>(
	url: string,
	options: FetchAnalysisOptions,
	parse: (payload: unknown) => Result,
): Promise<Result> {
	const headers: Record<string, string> = { Accept: "application/json" };
	const initData = options.initData?.trim();
	if (initData) {
		headers.Authorization = `tma ${initData}`;
	}

	try {
		const response = await (options.request ?? fetch)(url, {
			headers,
			method: "GET",
		});
		const payload: unknown = await response.json();
		if (!response.ok) {
			throw backendError(payload, response.status);
		}

		return parse(payload);
	} catch (error) {
		if (error instanceof ApiError) {
			throw error;
		}
		if (error instanceof TypeError) {
			throw new ApiError("network_error");
		}
		throw new ApiError("unexpected_error");
	}
}

function parseInstrumentAnalysisResult(
	payload: unknown,
): InstrumentAnalysisResult {
	if (
		!isRecord(payload) ||
		typeof payload.symbol !== "string" ||
		!isFiniteNumber(payload.period_days) ||
		!isFiniteNumber(payload.percentile) ||
		!isFiniteNumber(payload.range_percent) ||
		!isFiniteNumber(payload.candle_count) ||
		!isUtcDateTime(payload.from) ||
		!isUtcDateTime(payload.to)
	) {
		throw new ApiError("unexpected_error");
	}

	return {
		candle_count: payload.candle_count,
		from: payload.from,
		percentile: payload.percentile,
		period_days: payload.period_days,
		range_percent: payload.range_percent,
		symbol: payload.symbol,
		to: payload.to,
	};
}

function backendError(payload: unknown, status: number) {
	if (!isRecord(payload) || !isRecord(payload.error)) {
		return new ApiError("unexpected_error", { status });
	}

	const code = canonicalErrorCode(payload.error.code);
	return new ApiError(code, {
		requestId:
			typeof payload.request_id === "string" ? payload.request_id : undefined,
		status,
	});
}

function canonicalErrorCode(value: unknown): ApiErrorCode {
	switch (value) {
		case "access_denied":
		case "insufficient_data":
		case "invalid_argument":
		case "market_data_unavailable":
		case "symbol_not_found":
		case "unauthenticated":
			return value;
		default:
			return "unexpected_error";
	}
}

function parseMarketScanResult(payload: unknown): MarketScanResult {
	if (
		!isRecord(payload) ||
		!isFiniteNumber(payload.period_days) ||
		!isFiniteNumber(payload.percentile) ||
		!isFiniteNumber(payload.minimum_range_percent) ||
		!isFiniteNumber(payload.matched_count) ||
		!isFiniteNumber(payload.analyzed_count) ||
		!isFiniteNumber(payload.insufficient_data_count) ||
		!Array.isArray(payload.items)
	) {
		throw new ApiError("unexpected_error");
	}

	const items = payload.items.map(parseMarketScanItem);
	return {
		analyzed_count: payload.analyzed_count,
		insufficient_data_count: payload.insufficient_data_count,
		items,
		matched_count: payload.matched_count,
		minimum_range_percent: payload.minimum_range_percent,
		percentile: payload.percentile,
		period_days: payload.period_days,
	};
}

function parseMarketScanItem(payload: unknown): MarketScanItem {
	if (
		!isRecord(payload) ||
		typeof payload.symbol !== "string" ||
		!isFiniteNumber(payload.range_percent) ||
		!isFiniteNumber(payload.candle_count)
	) {
		throw new ApiError("unexpected_error");
	}

	return {
		candle_count: payload.candle_count,
		range_percent: payload.range_percent,
		symbol: payload.symbol,
	};
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null;
}

function isFiniteNumber(value: unknown): value is number {
	return typeof value === "number" && Number.isFinite(value);
}

function isUtcDateTime(value: unknown): value is string {
	return (
		typeof value === "string" &&
		(value.endsWith("Z") || value.endsWith("+00:00")) &&
		Number.isFinite(Date.parse(value))
	);
}
