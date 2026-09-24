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
	mergeChartCandlePages,
	mergeChartRsiPages,
	nextChartPageParam,
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
						getNextPageParam: nextChartPageParam,
						retry: false,
						staleTime: Number.POSITIVE_INFINITY,
					},
				},
			),
		);
	const listeners = new Set<() => void>();
	const live = createLiveStore(symbol.toUpperCase());
	type SnapshotInputs = {
		historical?: readonly ChartPageResponse[];
		head?: ChartPageResponse;
		live: ReturnType<typeof live.getSnapshot>;
		isLoading: boolean;
		isLoadingMore: boolean;
		hasMore: boolean;
		error?: string;
	};
	type IntervalState = {
		snapshot?: PriceHistorySnapshot;
		observer?: ReturnType<typeof createObserver>;
		pages?: readonly ChartPageResponse[];
		head?: ChartPageResponse;
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
			state = {};
			intervals.set(interval, state);
		}
		return state;
	};
	let connection: LiveCandlesClient | undefined;
	let unsubscribeLive: (() => void) | undefined;
	const cleanups: (() => void)[] = [];
	const recompute = (interval: CandleInterval) => {
		const current = forInterval(interval);
		const historical = current.pages;
		const head = current.head;
		const state = live.getSnapshot(interval);
		const query = current.observer?.getCurrentResult();
		const input = {
			historical,
			head,
			live: state,
			isLoading: query?.isPending ?? true,
			isLoadingMore: query?.isFetchingNextPage ?? false,
			hasMore: query?.hasNextPage ?? false,
			error: current.queryError ?? state.error,
		};
		const previous = current.lastInput;
		if (
			previous &&
			previous.historical === input.historical &&
			previous.head === input.head &&
			previous.live === input.live &&
			previous.isLoading === input.isLoading &&
			previous.isLoadingMore === input.isLoadingMore &&
			previous.hasMore === input.hasMore &&
			previous.error === input.error
		)
			return;
		current.lastInput = input;
		const candles = historical
			? mergeHistoryAndLiveCandles(
					mergeChartCandlePages(historical, head),
					state.candles,
				)
			: [];
		const indicator = mergeChartRsiPages(historical ?? [], head);
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
	const refresh = (interval: CandleInterval) => {
		const current = forInterval(interval);
		current.controller?.abort();
		const controller = new AbortController();
		current.controller = controller;
		void getInstrumentChart(
			symbol,
			rsiChartRequest,
			{ interval, limit: 200 },
			{
				...telegramRequestOptions(),
				signal: controller.signal,
			},
		)
			.then((response) => {
				if (controller.signal.aborted) return;
				const page = validateChartPage(response, symbol, interval);
				current.head = page;
				recompute(interval);
			})
			.catch(() => {});
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
						current.pages = result.data.pages.map((page) =>
							validateChartPage(page, symbol, interval),
						);
					} catch {
						validationError = "Price history is unavailable";
					}
				}
			}
			const error = result.isError
				? apiErrorMessage(result.error)
				: validationError;
			current.queryError = error;
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
				for (const timer of current.timers ?? []) clearTimeout(timer);
				current.timers = undefined;
			}
		},
		loadOlder(interval) {
			const selected = resolveInterval(interval);
			const observer = selected ? intervals.get(selected)?.observer : undefined;
			if (
				observer?.getCurrentResult().hasNextPage &&
				!observer.getCurrentResult().isFetchingNextPage
			)
				void observer.fetchNextPage();
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
