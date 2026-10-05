import { afterEach, expect, it, vi } from "vitest";

afterEach(() => {
	vi.unstubAllGlobals();
});

const initData = new URLSearchParams({
	user: JSON.stringify({ id: 1 }),
}).toString();

function setup() {
	const storage = new Map<string, string>();
	const setItem = vi.fn((key: string, value: string) => {
		storage.set(key, value);
	});
	vi.stubGlobal("window", {
		Telegram: { WebApp: { initData } },
		sessionStorage: {
			getItem: (key: string) => storage.get(key) ?? null,
			setItem,
			removeItem: (key: string) => storage.delete(key),
		},
	});
	return { setItem };
}

function deferredResponse() {
	let resolve!: (response: Response) => void;
	const promise = new Promise<Response>((complete) => {
		resolve = complete;
	});
	return { promise, resolve };
}

it("shares one exchange between concurrent calls for the same account", async () => {
	const { setItem } = setup();
	const response = deferredResponse();
	const fetchMock = vi.fn().mockReturnValue(response.promise);
	vi.stubGlobal("fetch", fetchMock);
	const { getSessionToken } = await import("@/api/session");
	const first = getSessionToken();
	const second = getSessionToken();
	expect(second).toBe(first);
	expect(fetchMock).toHaveBeenCalledOnce();
	expect(fetchMock).toHaveBeenCalledWith("/api/v1/auth/session", {
		headers: { Authorization: `tma ${initData}` },
		method: "POST",
	});
	response.resolve(Response.json({ token: "shared-token" }));
	await expect(Promise.all([first, second])).resolves.toEqual([
		"shared-token",
		"shared-token",
	]);
	expect(setItem).toHaveBeenCalledOnce();
});
