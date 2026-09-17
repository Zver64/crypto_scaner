import {
	MutationObserver,
	QueryClient,
	QueryObserver,
} from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { getAnalyzeMarketQueryKey } from "@/api/generated/api";
import type { MarketAnalysisResponse } from "@/api/generated/models";
import {
	fetchFreshMarketScan,
	marketScanMutationOptions,
	marketScanObserverOptions,
} from "@/features/market-scan/market-scan-screen/utils";
import {
	criterionSelections,
	defaultMarketScanCriteria,
	type MarketScanCriteria,
} from "@/features/market-scan/pipeline";
import {
	topCoinsCriteria,
	topCoinsRequestOptions,
} from "@/features/top-coins/top-coins";

const criteriaA = defaultMarketScanCriteria;
const criteriaB: MarketScanCriteria = {
	...defaultMarketScanCriteria,
	minimumMarketCapMillions: 250,
};

function response(matchedCount: number): MarketAnalysisResponse {
	return {
		analyzed_count: matchedCount,
		insufficient_data_count: 0,
		items: [],
		matched_count: matchedCount,
		price_history_window: {
			from: "2024-01-01T00:00:00.000Z",
			to: "2024-01-08T00:00:00.000Z",
		},
		unresolved: [],
		warnings: [],
	};
}

function ok(data: MarketAnalysisResponse) {
	return new Response(JSON.stringify(data), {
		headers: { "Content-Type": "application/json" },
		status: 200,
	});
}

function observerOptions(criteria: MarketScanCriteria, enabled = true) {
	return { ...marketScanObserverOptions(criteria), enabled };
}

function client() {
	return new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
}

async function waitForSettled<TQueryFnData, TError, TData>(
	observer: QueryObserver<TQueryFnData, TError, TData>,
) {
	const current = observer.getCurrentResult();
	if ((current.isSuccess || current.isError) && !current.isFetching) {
		return current;
	}
	return new Promise<ReturnType<typeof observer.getCurrentResult>>(
		(resolve) => {
			const unsubscribe = observer.subscribe((result) => {
				if ((result.isSuccess || result.isError) && !result.isFetching) {
					unsubscribe();
					resolve(result);
				}
			});
		},
	);
}

const clients: QueryClient[] = [];
const observers: Array<{ destroy(): void }> = [];

afterEach(() => {
	for (const observer of observers) observer.destroy();
	for (const queryClient of clients) queryClient.clear();
	observers.length = 0;
	clients.length = 0;
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

describe("market scan query policy", () => {
	it("fetches a restore miss once and restores a retained hit after more than five minutes", async () => {
		vi.useFakeTimers();
		const fetchMock = vi.fn(async () => ok(response(1)));
		vi.stubGlobal("fetch", fetchMock);
		const queryClient = client();
		clients.push(queryClient);

		const initialObserver = new QueryObserver(
			queryClient,
			observerOptions(criteriaA, false),
		);
		observers.push(initialObserver);
		const unsubscribeInitial = initialObserver.subscribe(() => undefined);
		expect(fetchMock).not.toHaveBeenCalled();
		initialObserver.setOptions(observerOptions(criteriaA));
		await waitForSettled(initialObserver);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		unsubscribeInitial();
		initialObserver.destroy();

		await vi.advanceTimersByTimeAsync(6 * 60 * 1000);
		const restoredObserver = new QueryObserver(
			queryClient,
			observerOptions(criteriaA),
		);
		observers.push(restoredObserver);
		const unsubscribeRestored = restoredObserver.subscribe(() => undefined);
		expect(restoredObserver.getCurrentResult().data?.matched_count).toBe(1);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		unsubscribeRestored();
	});

	it("runs every same-key submission while retaining one structural cache entry", async () => {
		const bodies: unknown[] = [];
		let run = 0;
		vi.stubGlobal(
			"fetch",
			vi.fn(async (_input, init: RequestInit) => {
				bodies.push(JSON.parse(String(init.body)));
				run += 1;
				return ok(response(run));
			}),
		);
		const queryClient = client();
		clients.push(queryClient);

		await fetchFreshMarketScan(queryClient, criteriaA);
		await fetchFreshMarketScan(queryClient, criteriaA);

		expect(bodies).toEqual([
			{ criteria: criterionSelections(criteriaA) },
			{ criteria: criterionSelections(criteriaA) },
		]);
		expect(
			queryClient.getQueryCache().findAll({
				queryKey: getAnalyzeMarketQueryKey({
					criteria: criterionSelections(criteriaA),
				}),
			}),
		).toHaveLength(1);
		expect(
			queryClient.getQueryData<{ data: MarketAnalysisResponse }>(
				getAnalyzeMarketQueryKey({ criteria: criterionSelections(criteriaA) }),
			)?.data.matched_count,
		).toBe(2);
	});

	it("runs A to B to cached A with exact submitted request bodies", async () => {
		const bodies: unknown[] = [];
		vi.stubGlobal(
			"fetch",
			vi.fn(async (_input, init: RequestInit) => {
				bodies.push(JSON.parse(String(init.body)));
				return ok(response(bodies.length));
			}),
		);
		const queryClient = client();
		clients.push(queryClient);

		await fetchFreshMarketScan(queryClient, criteriaA);
		await fetchFreshMarketScan(queryClient, criteriaB);
		await fetchFreshMarketScan(queryClient, criteriaA);

		expect(bodies).toEqual([
			{ criteria: criterionSelections(criteriaA) },
			{ criteria: criterionSelections(criteriaB) },
			{ criteria: criterionSelections(criteriaA) },
		]);
		expect(queryClient.getQueryCache().getAll()).toHaveLength(2);
	});

	it("switches the observer to a validated successful fetch without a duplicate request", async () => {
		const fetchMock = vi.fn(async () => ok(response(2)));
		vi.stubGlobal("fetch", fetchMock);
		const queryClient = client();
		clients.push(queryClient);
		const observer = new QueryObserver(
			queryClient,
			observerOptions(criteriaA, false),
		);
		observers.push(observer);
		const unsubscribe = observer.subscribe(() => undefined);

		await fetchFreshMarketScan(queryClient, criteriaB);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		observer.setOptions(observerOptions(criteriaB));
		await Promise.resolve();
		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(observer.getCurrentResult().data?.matched_count).toBe(2);
		unsubscribe();
	});

	it("does not invalidate or overwrite the distinct top-coins cache", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn(async () => ok(response(1))),
		);
		const queryClient = client();
		clients.push(queryClient);
		const topRequest = {
			criteria: [...topCoinsCriteria],
			...topCoinsRequestOptions,
		};
		const topData = { data: response(10), headers: new Headers(), status: 200 };
		queryClient.setQueryData(getAnalyzeMarketQueryKey(topRequest), topData);

		await fetchFreshMarketScan(queryClient, criteriaA);

		expect(queryClient.getQueryData(getAnalyzeMarketQueryKey(topRequest))).toBe(
			topData,
		);
	});

	it("retains data with a native refetch error, restores without fetching, and permits retry", async () => {
		const failure = new Error("offline");
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(ok(response(1)))
			.mockRejectedValueOnce(failure)
			.mockResolvedValueOnce(ok(response(3)));
		vi.stubGlobal("fetch", fetchMock);
		const queryClient = client();
		clients.push(queryClient);
		const initialObserver = new QueryObserver(
			queryClient,
			observerOptions(criteriaA),
		);
		observers.push(initialObserver);
		const unsubscribeInitial = initialObserver.subscribe(() => undefined);
		await waitForSettled(initialObserver);

		await expect(fetchFreshMarketScan(queryClient, criteriaA)).rejects.toThrow(
			"offline",
		);
		const failedResult = initialObserver.getCurrentResult();
		expect(fetchMock).toHaveBeenCalledTimes(2);
		expect(failedResult.data?.matched_count).toBe(1);
		expect(failedResult.error).toBe(failure);
		expect(failedResult.isRefetchError).toBe(true);

		unsubscribeInitial();
		initialObserver.destroy();
		const restoredObserver = new QueryObserver(
			queryClient,
			observerOptions(criteriaA),
		);
		observers.push(restoredObserver);
		const unsubscribeRestored = restoredObserver.subscribe(() => undefined);
		await Promise.resolve();
		expect(fetchMock).toHaveBeenCalledTimes(2);
		expect(restoredObserver.getCurrentResult().data?.matched_count).toBe(1);
		expect(restoredObserver.getCurrentResult().isRefetchError).toBe(true);

		await fetchFreshMarketScan(queryClient, criteriaA);
		expect(fetchMock).toHaveBeenCalledTimes(3);
		expect(restoredObserver.getCurrentResult().data?.matched_count).toBe(3);
		unsubscribeRestored();
	});

	it("never replaces valid data with a semantic-invalid response", async () => {
		const invalid = response(2);
		invalid.price_history_window.to = invalid.price_history_window.from;
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(ok(response(1)))
			.mockResolvedValueOnce(ok(invalid));
		vi.stubGlobal("fetch", fetchMock);
		const queryClient = client();
		clients.push(queryClient);
		const observer = new QueryObserver(queryClient, observerOptions(criteriaA));
		observers.push(observer);
		const unsubscribe = observer.subscribe(() => undefined);
		await waitForSettled(observer);

		await expect(
			fetchFreshMarketScan(queryClient, criteriaA),
		).rejects.toThrow();
		expect(fetchMock).toHaveBeenCalledTimes(2);
		expect(observer.getCurrentResult().data?.matched_count).toBe(1);
		expect(observer.getCurrentResult().isRefetchError).toBe(true);
		unsubscribe();
	});

	it("does not commit failed changed criteria and commits a later success", async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(ok(response(1)))
			.mockRejectedValueOnce(new Error("offline"))
			.mockResolvedValueOnce(ok(response(3)));
		vi.stubGlobal("fetch", fetchMock);
		const queryClient = client();
		clients.push(queryClient);
		await fetchFreshMarketScan(queryClient, criteriaA);
		let committedCriteria = criteriaA;
		const onCriteriaCommit = vi.fn();
		const mutation = new MutationObserver(
			queryClient,
			marketScanMutationOptions(queryClient),
		);
		const unsubscribe = mutation.subscribe(() => undefined);
		const onError = vi.fn();
		const commit = {
			onError,
			onSuccess: (_response: unknown, criteria: MarketScanCriteria) => {
				committedCriteria = criteria;
				onCriteriaCommit(criteria);
			},
		};

		await expect(mutation.mutate(criteriaB, commit)).rejects.toThrow("offline");
		expect(committedCriteria).toBe(criteriaA);
		expect(onCriteriaCommit).not.toHaveBeenCalled();
		expect(onError).toHaveBeenCalledOnce();

		await mutation.mutate(criteriaB, commit);
		expect(committedCriteria).toBe(criteriaB);
		expect(onCriteriaCommit).toHaveBeenCalledOnce();
		expect(onCriteriaCommit).toHaveBeenCalledWith(criteriaB);
		unsubscribe();
	});

	it("populates the canonical cache without running observer callbacks after unsubscribe", async () => {
		let resolveFetch!: (response: Response) => void;
		const fetchMock = vi.fn(
			() =>
				new Promise<Response>((resolve) => {
					resolveFetch = resolve;
				}),
		);
		vi.stubGlobal("fetch", fetchMock);
		const queryClient = client();
		clients.push(queryClient);
		const mutation = new MutationObserver(
			queryClient,
			marketScanMutationOptions(queryClient),
		);
		const unsubscribe = mutation.subscribe(() => undefined);
		const onSuccess = vi.fn();
		const promise = mutation.mutate(criteriaB, { onSuccess });

		await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledOnce());
		unsubscribe();
		resolveFetch(ok(response(4)));
		await promise;

		expect(
			queryClient.getQueryData<{ data: MarketAnalysisResponse }>(
				getAnalyzeMarketQueryKey({ criteria: criterionSelections(criteriaB) }),
			)?.data.matched_count,
		).toBe(4);
		expect(onSuccess).not.toHaveBeenCalled();
	});

	it("does not run an observer error callback after unsubscribe", async () => {
		let rejectFetch!: (error: Error) => void;
		const fetchMock = vi.fn(
			() =>
				new Promise<Response>((_resolve, reject) => {
					rejectFetch = reject;
				}),
		);
		vi.stubGlobal("fetch", fetchMock);
		const queryClient = client();
		clients.push(queryClient);
		const mutation = new MutationObserver(
			queryClient,
			marketScanMutationOptions(queryClient),
		);
		const unsubscribe = mutation.subscribe(() => undefined);
		const onError = vi.fn();
		const promise = mutation.mutate(criteriaB, { onError });

		await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledOnce());
		unsubscribe();
		rejectFetch(new Error("offline"));
		await expect(promise).rejects.toThrow("offline");
		expect(onError).not.toHaveBeenCalled();
	});

	it("settles a structurally identical success through native query and mutation state", async () => {
		const identical = response(1);
		vi.stubGlobal(
			"fetch",
			vi.fn(async () => ok(identical)),
		);
		const queryClient = client();
		clients.push(queryClient);
		const observer = new QueryObserver(queryClient, observerOptions(criteriaA));
		observers.push(observer);
		const queryTransitions: boolean[] = [];
		const unsubscribeQuery = observer.subscribe((result) => {
			queryTransitions.push(result.isFetching);
		});
		await waitForSettled(observer);
		const firstData = observer.getCurrentResult().data;
		const mutation = new MutationObserver(
			queryClient,
			marketScanMutationOptions(queryClient),
		);
		const mutationTransitions: boolean[] = [];
		const unsubscribeMutation = mutation.subscribe((result) => {
			mutationTransitions.push(result.isPending);
		});

		await mutation.mutate(criteriaA);

		expect(queryTransitions.slice(-2)).toEqual([true, false]);
		expect(mutationTransitions).toEqual([true, false]);
		expect(observer.getCurrentResult().data).toBe(firstData);
		expect(mutation.getCurrentResult().isSuccess).toBe(true);
		unsubscribeMutation();
		unsubscribeQuery();
	});
});
