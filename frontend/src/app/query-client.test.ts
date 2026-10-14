import { notifications } from "@mantine/notifications";
import type { QueryMeta } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import { createQueryClient } from "@/app/query-client";
import { scopedUserQueryKey } from "@/features/favorites/user-query-scope";

vi.mock("@mantine/notifications", () => ({
	notifications: { show: vi.fn<typeof notifications.show>() },
}));

function apiError(status: number, code?: string) {
	return Object.assign(new Error("failed"), {
		info: code ? { error: { code } } : undefined,
		status,
	});
}

async function fail(
	queryClient: ReturnType<typeof createQueryClient>,
	queryKey: readonly unknown[],
	meta: QueryMeta,
	error: Error,
) {
	await queryClient
		.fetchQuery({
			meta,
			queryFn: () => Promise.reject(error),
			queryKey,
			retry: false,
		})
		.catch(() => undefined);
}

const errorNotification = { id: "scan-error", title: "Scan failed" };

afterEach(() => {
	vi.clearAllMocks();
	vi.useRealTimers();
});

describe("createQueryClient", () => {
	it("notifies from query meta, except ignored codes and quiet refetches", async () => {
		const queryClient = createQueryClient();
		await fail(queryClient, ["a"], { errorNotification }, apiError(500));
		expect(notifications.show).toHaveBeenCalledWith(
			expect.objectContaining({ id: "scan-error", title: "Scan failed" }),
		);

		vi.mocked(notifications.show).mockClear();
		await fail(
			queryClient,
			["b"],
			{
				errorNotification: {
					...errorNotification,
					ignoredCode: "insufficient_data",
				},
			},
			apiError(400, "insufficient_data"),
		);
		queryClient.setQueryData(["c"], 1);
		await fail(
			queryClient,
			["c"],
			{ errorNotification: { ...errorNotification, initialLoadOnly: true } },
			apiError(500),
		);
		expect(notifications.show).not.toHaveBeenCalled();
	});

	it("drops the user's queries on an access error and other users' queries on a new one", async () => {
		vi.useFakeTimers();
		const queryClient = createQueryClient();
		const kept = ["other"];
		queryClient.setQueryData(kept, 1);
		queryClient.setQueryData(scopedUserQueryKey(["alerts"], "user:1"), 1);
		queryClient.setQueryData(scopedUserQueryKey(["favorites"], "user:2"), 1);
		await vi.runAllTimersAsync();
		expect(queryClient.getQueryCache().getAll()).toHaveLength(2);

		const favorites = scopedUserQueryKey(["favorites"]);
		queryClient.setQueryData(favorites, 1);
		await fail(queryClient, favorites, { userScoped: true }, apiError(403));
		await vi.runAllTimersAsync();
		expect(
			queryClient
				.getQueryCache()
				.getAll()
				.map(({ queryKey }) => queryKey),
		).toEqual([kept]);
	});
});
