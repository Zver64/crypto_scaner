export interface CriterionSelection {
	name: string;
	parameters: Record<string, unknown>;
}

export interface Evaluation {
	candle_count: number;
	from: string;
	matched: boolean;
	metrics: Record<string, number>;
	name: string;
	to: string;
}

export interface InstrumentAnalysisResult {
	evaluations: Evaluation[];
	matched: boolean;
	symbol: string;
	warnings: Warning[];
}

export interface MarketScanItem
	extends Omit<InstrumentAnalysisResult, "warnings"> {}

export type UnresolvedInstrumentCode =
	| "mapping_conflict"
	| "mapping_not_found"
	| "mapping_provider_unavailable"
	| "market_cap_missing";

export interface UnresolvedInstrument {
	code: UnresolvedInstrumentCode;
	message: string;
	symbol: string;
}

export interface Warning {
	code: string;
	message: string;
}

export interface MarketScanResult {
	analyzed_count: number;
	insufficient_data_count: number;
	items: MarketScanItem[];
	matched_count: number;
	unresolved: UnresolvedInstrument[];
	warnings: Warning[];
}

type ApiErrorCode =
	| "access_denied"
	| "insufficient_data"
	| "invalid_argument"
	| "market_data_unavailable"
	| "market_cap_unavailable"
	| "network_error"
	| "symbol_not_found"
	| "unauthenticated"
	| "unexpected_error";

const rfc3339UtcDateTime =
	/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?Z$/;

const apiErrorMessages: Record<ApiErrorCode, string> = {
	access_denied: "Your Telegram account does not have access to this scanner.",
	insufficient_data: "There is not enough market history for this analysis.",
	invalid_argument: "The request contains unsupported analysis criteria.",
	market_data_unavailable: "Market data is not ready yet. Try again shortly.",
	market_cap_unavailable:
		"Market capitalization is unavailable for this instrument or is still loading. Try again shortly.",
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
	criteria: readonly CriterionSelection[],
	options: FetchAnalysisOptions = {},
): Promise<MarketScanResult> {
	return fetchAnalysisResult(
		"/api/v1/analysis/market",
		criteria,
		options,
		parseMarketScanResult,
	);
}

export async function fetchInstrumentAnalysis(
	symbol: string,
	criteria: readonly CriterionSelection[],
	options: FetchAnalysisOptions = {},
): Promise<InstrumentAnalysisResult> {
	return fetchAnalysisResult(
		`/api/v1/analysis/instruments/${encodeURIComponent(symbol)}`,
		criteria,
		options,
		parseInstrumentAnalysisResult,
	);
}

async function fetchAnalysisResult<Result>(
	url: string,
	criteria: readonly CriterionSelection[],
	options: FetchAnalysisOptions,
	parse: (payload: unknown) => Result,
): Promise<Result> {
	const headers: Record<string, string> = {
		Accept: "application/json",
		"Content-Type": "application/json",
	};
	const initData = options.initData?.trim();
	if (initData) {
		headers.Authorization = `tma ${initData}`;
	}

	try {
		const response = await (options.request ?? fetch)(url, {
			body: JSON.stringify({ criteria }),
			headers,
			method: "POST",
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
		!isBoolean(payload.matched) ||
		!Array.isArray(payload.evaluations) ||
		!Array.isArray(payload.warnings)
	) {
		throw new ApiError("unexpected_error");
	}

	return {
		evaluations: payload.evaluations.map(parseEvaluation),
		matched: payload.matched,
		symbol: payload.symbol,
		warnings: payload.warnings.map(parseWarning),
	};
}

function backendError(payload: unknown, status: number) {
	if (!isRecord(payload) || !isRecord(payload.error)) {
		return new ApiError("unexpected_error", { status });
	}

	const code =
		status === 422
			? "market_cap_unavailable"
			: canonicalErrorCode(payload.error.code);
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
		case "market_cap_unavailable":
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
		!isFiniteNumber(payload.matched_count) ||
		!isFiniteNumber(payload.analyzed_count) ||
		!isFiniteNumber(payload.insufficient_data_count) ||
		!Array.isArray(payload.items) ||
		!Array.isArray(payload.unresolved) ||
		!Array.isArray(payload.warnings)
	) {
		throw new ApiError("unexpected_error");
	}

	const items = payload.items.map(parseMarketScanItem);
	return {
		analyzed_count: payload.analyzed_count,
		insufficient_data_count: payload.insufficient_data_count,
		items,
		matched_count: payload.matched_count,
		unresolved: payload.unresolved.map(parseUnresolvedInstrument),
		warnings: payload.warnings.map(parseWarning),
	};
}

function parseUnresolvedInstrument(payload: unknown): UnresolvedInstrument {
	if (
		!isRecord(payload) ||
		typeof payload.symbol !== "string" ||
		typeof payload.code !== "string" ||
		typeof payload.message !== "string"
	) {
		throw new ApiError("unexpected_error");
	}

	return {
		code: parseUnresolvedInstrumentCode(payload.code),
		message: payload.message,
		symbol: payload.symbol,
	};
}

function parseUnresolvedInstrumentCode(
	value: unknown,
): UnresolvedInstrumentCode {
	switch (value) {
		case "mapping_conflict":
		case "mapping_not_found":
		case "mapping_provider_unavailable":
		case "market_cap_missing":
			return value;
		default:
			throw new ApiError("unexpected_error");
	}
}

function parseWarning(payload: unknown): Warning {
	if (
		!isRecord(payload) ||
		typeof payload.code !== "string" ||
		typeof payload.message !== "string"
	) {
		throw new ApiError("unexpected_error");
	}

	return { code: payload.code, message: payload.message };
}

function parseMarketScanItem(payload: unknown): MarketScanItem {
	if (
		!isRecord(payload) ||
		typeof payload.symbol !== "string" ||
		!isBoolean(payload.matched) ||
		!Array.isArray(payload.evaluations)
	) {
		throw new ApiError("unexpected_error");
	}

	return {
		evaluations: payload.evaluations.map(parseEvaluation),
		matched: payload.matched,
		symbol: payload.symbol,
	};
}

function parseEvaluation(payload: unknown): Evaluation {
	if (
		!isRecord(payload) ||
		typeof payload.name !== "string" ||
		!isBoolean(payload.matched) ||
		!isRecord(payload.metrics) ||
		!isFiniteNumber(payload.candle_count) ||
		!isRfc3339UtcDateTime(payload.from) ||
		!isRfc3339UtcDateTime(payload.to)
	) {
		throw new ApiError("unexpected_error");
	}

	const metrics: Record<string, number> = {};
	for (const [name, value] of Object.entries(payload.metrics)) {
		if (!isFiniteNumber(value)) {
			throw new ApiError("unexpected_error");
		}
		metrics[name] = value;
	}

	return {
		candle_count: payload.candle_count,
		from: payload.from,
		matched: payload.matched,
		metrics,
		name: payload.name,
		to: payload.to,
	};
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null;
}

function isFiniteNumber(value: unknown): value is number {
	return typeof value === "number" && Number.isFinite(value);
}

function isBoolean(value: unknown): value is boolean {
	return typeof value === "boolean";
}

function isRfc3339UtcDateTime(value: unknown): value is string {
	if (typeof value !== "string") {
		return false;
	}

	const parts = rfc3339UtcDateTime.exec(value);
	const timestamp = Date.parse(value);
	if (!parts || !Number.isFinite(timestamp)) {
		return false;
	}

	const date = new Date(timestamp);
	return (
		date.getUTCFullYear() === Number(parts[1]) &&
		date.getUTCMonth() + 1 === Number(parts[2]) &&
		date.getUTCDate() === Number(parts[3]) &&
		date.getUTCHours() === Number(parts[4]) &&
		date.getUTCMinutes() === Number(parts[5]) &&
		date.getUTCSeconds() === Number(parts[6])
	);
}
