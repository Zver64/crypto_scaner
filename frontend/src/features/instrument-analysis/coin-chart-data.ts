import { InfiniteQueryObserver, type QueryClient } from "@tanstack/react-query";
import {
	getGetInstrumentChartInfiniteQueryOptions,
	getInstrumentChart,
} from "@/api/generated/api";
import type { CandleInterval, ChartPageResponse } from "@/api/generated/models";
import { LiveCandlesClient } from "@/api/live-candles";
import { getTelegramInitData, telegramRequestOptions } from "@/app/telegram";
import type {
	PriceHistorySnapshot,
	PriceHistorySource,
} from "@/components/price-history-chart";
import { apiErrorMessage } from "@/features/analysis/api-error";
import {
	rsiChartRequest,
	validateChartPage,
} from "@/features/instrument-analysis/chart-page";
import { mergeHistoryAndLiveCandles } from "@/features/instrument-analysis/live-candle-merge";
import {
	chartIntervals,
	createLiveStore,
} from "@/features/instrument-analysis/live-candle-store";

export function createCoinChartData(
	symbol: string,
	queryClient: QueryClient,
): PriceHistorySource {
	const createObserver = (interval: CandleInterval) =>
		new InfiniteQueryObserver(
			queryClient,
			getGetInstrumentChartInfiniteQueryOptions(
				symbol,
				rsiChartRequest,
				{ interval, limit: 200 },
				{
					fetch: telegramRequestOptions(),
					query: {
						initialPageParam: undefined,
						getNextPageParam: () => undefined,
						retry: false,
						staleTime: Number.POSITIVE_INFINITY,
					},
				},
			),
		);
	const listeners = new Set<() => void>();
	const live = createLiveStore(symbol.toUpperCase());
	type SnapshotInputs = {
		historical?: ChartPageResponse;
		live: ReturnType<typeof live.getSnapshot>;
		isLoading: boolean;
		isLoadingMore: boolean;
		hasMore: boolean;
		error?: string;
	};
	type IntervalState = {
		snapshot?: PriceHistorySnapshot;
		observer?: ReturnType<typeof createObserver>;
		page?: ChartPageResponse;
		limit: number;
		loadingMore?: boolean;
		pendingRefresh?: boolean;
		generation: number;
		queryError?: string;
		controller?: AbortController;
		timers?: ReturnType<typeof setTimeout>[];
		closure?: string;
		lastInput?: SnapshotInputs;
	};
	const intervals = new Map<CandleInterval, IntervalState>();
	const resolveInterval = (value: string) =>
		chartIntervals.find((interval) => interval === value);
	const forInterval = (interval: CandleInterval): IntervalState => {
		let state = intervals.get(interval);
		if (!state) {
			state = { limit: 200, generation: 0 };
			intervals.set(interval, state);
		}
		return state;
	};
	let connection: LiveCandlesClient | undefined;
	let unsubscribeLive: (() => void) | undefined;
	const cleanups: (() => void)[] = [];
	const recompute = (interval: CandleInterval) => {
		const current = forInterval(interval);
		const historical = current.page;
		const state = live.getSnapshot(interval);
		const query = current.observer?.getCurrentResult();
		const input = {
			historical,
			live: state,
			isLoading: query?.isPending ?? true,
			isLoadingMore: current.loadingMore ?? false,
			hasMore: (historical?.has_more ?? false) && current.limit < 5000,
			error: current.queryError ?? state.error,
		};
		const previous = current.lastInput;
		if (
			previous &&
			previous.historical === input.historical &&
			previous.live === input.live &&
			previous.isLoading === input.isLoading &&
			previous.isLoadingMore === input.isLoadingMore &&
			previous.hasMore === input.hasMore &&
			previous.error === input.error
		)
			return;
		current.lastInput = input;
		const candles = historical
			? mergeHistoryAndLiveCandles(historical.candles, state.candles)
			: [];
		const indicator = historical?.indicators[0]?.series[0]?.points ?? [];
		const next: PriceHistorySnapshot = {
			candles,
			indicator,
			connection: state.connection,
			freshness: state.freshness,
			error: input.error,
			isLoading: input.isLoading,
			isLoadingMore: input.isLoadingMore,
			hasMore: input.hasMore,
		};
		current.snapshot = next;
		for (const listener of listeners) listener();
	};
	// An initial chart stays at 200 closed candles. Once extended, retain its
	// oldest candle across closures (up to the bounded 5000-candle range).
	// Each response replaces the entire range, including all RSI points.
	const fetchRange = (
		interval: CandleInterval,
		requestedLimit: number,
		older: boolean,
	) => {
		const current = forInterval(interval);
		current.controller?.abort();
		const generation = ++current.generation;
		const controller = new AbortController();
		current.controller = controller;
		const oldest = current.page?.candles[0]?.open_time;
		const newest = current.page?.candles.at(-1)?.open_time;
		if (older) current.loadingMore = true;
		recompute(interval);
		void (async () => {
			let limit = requestedLimit;
			while (true) {
				const response = await getInstrumentChart(
					symbol,
					rsiChartRequest,
					{ interval, limit },
					{
						...telegramRequestOptions(),
						signal: controller.signal,
					},
				);
				if (controller.signal.aborted || generation !== current.generation)
					return;
				const page = validateChartPage(response, symbol, interval);
				if (
					!oldest ||
					!page.candles.length ||
					(!older && current.limit === 200) ||
					(older
						? page.candles[0].open_time < oldest
						: page.candles[0].open_time <= oldest) ||
					!page.has_more ||
					limit === 5000
				) {
					current.page = page;
					current.limit = limit;
					current.queryError = undefined;
					return;
				}
				// Count newly closed candles rather than adding another whole page
				// of older history on every refresh. If the old head fell outside
				// the response, grow in bounded steps until it becomes visible.
				const oldHeadIndex = page.candles.findIndex(
					(candle) => candle.open_time === newest,
				);
				const newCandles =
					oldHeadIndex < 0 ? 200 : page.candles.length - oldHeadIndex - 1;
				limit = Math.min(5000, limit + Math.max(1, newCandles));
			}
		})()
			.catch((error) => {
				if (!controller.signal.aborted && generation === current.generation)
					current.queryError = apiErrorMessage(error);
			})
			.finally(() => {
				if (generation !== current.generation) return;
				current.loadingMore = false;
				recompute(interval);
				if (current.pendingRefresh) {
					current.pendingRefresh = false;
					refresh(interval);
				}
			});
	};
	const refresh = (interval: CandleInterval) => {
		const current = forInterval(interval);
		if (current.loadingMore) {
			current.pendingRefresh = true;
			return;
		}
		fetchRange(interval, current.limit, false);
	};
	const scheduleRecovery = (interval: CandleInterval, recovering: boolean) => {
		const current = forInterval(interval);
		for (const timer of current.timers ?? []) clearTimeout(timer);
		current.timers = (recovering ? [0, 1000, 2000, 4000, 8000] : [0]).map(
			(delay) => setTimeout(() => refresh(interval), delay),
		);
	};
	let initialInterval: CandleInterval | undefined;
	const startObserver = (interval: CandleInterval) => {
		const current = forInterval(interval);
		if (current.observer) return;
		const observer = createObserver(interval);
		current.observer = observer;
		let lastResult: ReturnType<typeof observer.getCurrentResult> | undefined;
		let lastData: ReturnType<typeof observer.getCurrentResult>["data"];
		let validationError: string | undefined;
		let warmed = false;
		const handleResult = (
			result: ReturnType<typeof observer.getCurrentResult>,
		) => {
			if (result === lastResult) return;
			lastResult = result;
			if (result.data !== lastData) {
				lastData = result.data;
				validationError = undefined;
				if (result.data) {
					try {
						if (current.limit === 200 && !current.page) {
							current.page = validateChartPage(
								result.data.pages[0],
								symbol,
								interval,
							);
							current.queryError = undefined;
						}
					} catch {
						validationError = "Price history is unavailable";
					}
				}
			}
			// A late initial-query failure cannot replace the status of a newer
			// successfully loaded range (or a later range-fetch error).
			if (!current.page)
				current.queryError = result.isError
					? apiErrorMessage(result.error)
					: validationError;
			recompute(interval);
			if (result.data && interval === initialInterval && !warmed) {
				warmed = true;
				queueMicrotask(() => {
					if (
						connection &&
						current.observer === observer &&
						initialInterval === interval
					)
						for (const other of chartIntervals) startObserver(other);
				});
			}
		};
		cleanups.push(observer.subscribe(handleResult));
		handleResult(observer.getCurrentResult());
	};
	return {
		getSnapshot(interval) {
			const selected = resolveInterval(interval);
			return selected
				? (intervals.get(selected)?.snapshot ?? emptySnapshot)
				: emptySnapshot;
		},
		subscribe(listener) {
			listeners.add(listener);
			return () => {
				listeners.delete(listener);
			};
		},
		start(interval) {
			const selected = resolveInterval(interval);
			if (connection || !selected) return;
			live.connection("connecting");
			initialInterval = selected;
			startObserver(selected);
			unsubscribeLive = live.subscribe(() => {
				for (const interval of chartIntervals) {
					const states = live.getSnapshot(interval);
					const closure = [...states.candles]
						.reverse()
						.find((item) => item.final)?.candle.open_time;
					const current = forInterval(interval);
					if (closure && current.closure !== closure) {
						current.closure = closure;
						scheduleRecovery(interval, false);
					} else if (
						states.freshness === "recovering" &&
						current.snapshot?.freshness !== "recovering"
					) {
						scheduleRecovery(interval, true);
					} else if (
						current.snapshot?.freshness === "recovering" &&
						states.freshness !== "recovering"
					) {
						scheduleRecovery(interval, false);
					}
					recompute(interval);
				}
			});
			connection = new LiveCandlesClient({
				getInitData: getTelegramInitData,
				onConnectionChange: live.connection,
				onMessage: live.message,
			});
			connection.setSubscriptions(
				chartIntervals.map((interval) => ({
					symbol: symbol.toUpperCase(),
					interval,
				})),
			);
			connection.connect();
		},
		select(interval) {
			const selected = resolveInterval(interval);
			if (connection && selected) startObserver(selected);
		},
		stop() {
			connection?.disconnect();
			connection = undefined;
			live.connection("disconnected");
			unsubscribeLive?.();
			unsubscribeLive = undefined;
			for (const cleanup of cleanups.splice(0)) cleanup();
			for (const current of intervals.values()) {
				current.observer = undefined;
				current.lastInput = undefined;
				current.controller?.abort();
				current.controller = undefined;
				current.generation++;
				current.loadingMore = false;
				current.pendingRefresh = false;
				for (const timer of current.timers ?? []) clearTimeout(timer);
				current.timers = undefined;
			}
		},
		loadOlder(interval) {
			const selected = resolveInterval(interval);
			if (!selected) return;
			const current = forInterval(selected);
			if (
				!current.page?.has_more ||
				current.loadingMore ||
				current.limit >= 5000
			)
				return;
			fetchRange(selected, Math.min(5000, current.limit + 200), true);
		},
	};
}

const emptySnapshot: PriceHistorySnapshot = {
	candles: [],
	indicator: [],
	connection: "connecting",
	freshness: "waiting",
	isLoading: true,
	isLoadingMore: false,
	hasMore: false,
};
