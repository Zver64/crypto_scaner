import { afterEach, expect, it, vi } from "vitest";

import { LiveCandlesClient } from "@/api/live-candles";

class FakeSocket {
	static OPEN = 1;
	static instances: FakeSocket[] = [];
	readyState = FakeSocket.OPEN;
	messages: unknown[] = [];
	listeners = new Map<string, ((event: { data?: string }) => void)[]>();

	constructor(
		_url: string,
		readonly protocols: string[],
	) {
		// Browsers reject a subprotocol that is not an HTTP token.
		if (
			protocols.some((protocol) => !/^[!#$%&'*+.^_`|~\w-]+$/.test(protocol))
		) {
			throw new DOMException("Invalid subprotocol", "SyntaxError");
		}
		FakeSocket.instances.push(this);
	}

	addEventListener(type: string, listener: (event: { data?: string }) => void) {
		this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
	}

	emit(type: string, data?: string) {
		for (const listener of this.listeners.get(type) ?? []) listener({ data });
	}

	send(message: string) {
		this.messages.push(JSON.parse(message));
	}

	close() {
		this.readyState = 3;
		this.emit("close");
	}
}

function firstSocket(): FakeSocket {
	const socket = FakeSocket.instances[0];
	if (!socket) throw new Error("The client opened no socket.");
	return socket;
}

afterEach(() => {
	vi.unstubAllGlobals();
	FakeSocket.instances = [];
});

it("subscribes to every chart interval on one socket and retains them while switching views", async () => {
	vi.stubGlobal("window", {
		location: { href: "https://example.com/instruments/BTC" },
	});
	vi.stubGlobal("WebSocket", FakeSocket);
	const client = new LiveCandlesClient({
		getToken: async () => "session-token",
		invalidateToken: () => {},
		checkSession: async () => undefined,
		onConnectionChange: () => {},
		onMessage: () => {},
	});
	const subscriptions = (["1h", "1d", "1w", "1M"] as const).map((interval) => ({
		symbol: "BTC",
		interval,
	}));
	client.setSubscriptions(subscriptions);
	client.connect();
	await vi.waitFor(() => expect(FakeSocket.instances).toHaveLength(1));
	const socket = firstSocket();
	// The handshake carries the session token, never a message or the URL.
	expect(socket.protocols).toEqual(["crypto-scanner.live.v1", "session-token"]);
	socket.emit("open");
	expect(socket.messages).toEqual(
		subscriptions.map((subscription) => ({
			type: "subscribe",
			...subscription,
		})),
	);

	client.setSubscriptions(subscriptions);
	expect(FakeSocket.instances).toHaveLength(1);
	expect(socket.messages).toHaveLength(4);
	client.disconnect();
});

it("updates only changed subscriptions without reopening the socket", async () => {
	vi.stubGlobal("window", { location: { href: "https://example.com/" } });
	vi.stubGlobal("WebSocket", FakeSocket);
	const client = new LiveCandlesClient({
		getToken: async () => "session-token",
		invalidateToken: () => {},
		checkSession: async () => undefined,
		onConnectionChange: () => {},
		onMessage: () => {},
	});
	client.setSubscriptions([{ symbol: "BTC", interval: "1h" }]);
	client.connect();
	await vi.waitFor(() => expect(FakeSocket.instances).toHaveLength(1));
	const socket = firstSocket();
	socket.emit("open");
	expect(socket.messages).toHaveLength(1);
	client.setSubscriptions([{ symbol: "ETH", interval: "1h" }]);
	expect(socket.messages.slice(1)).toEqual([
		{ type: "unsubscribe", symbol: "BTC", interval: "1h" },
		{ type: "subscribe", symbol: "ETH", interval: "1h" },
	]);
	expect(FakeSocket.instances).toHaveLength(1);
	client.disconnect();
});

it("checks the session after a failed handshake and reconnects with the renewed token", async () => {
	vi.useFakeTimers();
	vi.stubGlobal("window", { location: { href: "https://example.com/" } });
	vi.stubGlobal("WebSocket", FakeSocket);
	let token = "expired-token";
	const checkSession = vi.fn<() => Promise<void>>(async () => {
		token = "renewed-token";
	});
	const client = new LiveCandlesClient({
		getToken: async () => token,
		invalidateToken: () => {},
		checkSession,
		onConnectionChange: () => {},
		onMessage: () => {},
	});
	try {
		client.connect();
		await vi.waitFor(() => expect(FakeSocket.instances).toHaveLength(1));
		// The handshake was rejected: the socket closes without opening.
		firstSocket().close();
		await vi.waitFor(() => expect(checkSession).toHaveBeenCalledOnce());
		await vi.advanceTimersByTimeAsync(1_000);
		expect(FakeSocket.instances[1]?.protocols).toEqual([
			"crypto-scanner.live.v1",
			"renewed-token",
		]);

		// A socket that opened and then closed reconnects without a check.
		FakeSocket.instances[1]?.emit("open");
		FakeSocket.instances[1]?.close();
		await vi.advanceTimersByTimeAsync(1_000);
		expect(FakeSocket.instances).toHaveLength(3);
		expect(checkSession).toHaveBeenCalledOnce();
	} finally {
		client.disconnect();
		vi.useRealTimers();
	}
});

it("keeps growing the reconnect delay while handshakes fail", async () => {
	vi.useFakeTimers();
	vi.stubGlobal("window", { location: { href: "https://example.com/" } });
	vi.stubGlobal("WebSocket", FakeSocket);
	const client = new LiveCandlesClient({
		getToken: async () => "session-token",
		invalidateToken: () => {},
		checkSession: async () => undefined,
		onConnectionChange: () => {},
		onMessage: () => {},
	});
	try {
		client.connect();
		await vi.waitFor(() => expect(FakeSocket.instances).toHaveLength(1));
		for (const delay of [1_000, 2_000, 4_000]) {
			const count = FakeSocket.instances.length;
			// The server rejected the handshake, for example at a connection limit.
			FakeSocket.instances[count - 1]?.close();
			await vi.advanceTimersByTimeAsync(delay - 1);
			expect(FakeSocket.instances).toHaveLength(count);
			await vi.advanceTimersByTimeAsync(1);
			expect(FakeSocket.instances).toHaveLength(count + 1);
		}
	} finally {
		client.disconnect();
		vi.useRealTimers();
	}
});

it("forgets a token that is not a valid subprotocol and reconnects with a new one", async () => {
	vi.useFakeTimers();
	vi.stubGlobal("window", { location: { href: "https://example.com/" } });
	vi.stubGlobal("WebSocket", FakeSocket);
	let token = "malformed token";
	const invalidateToken = vi.fn<(token: string) => void>(() => {
		token = "renewed-token";
	});
	const connections: string[] = [];
	const client = new LiveCandlesClient({
		getToken: async () => token,
		invalidateToken,
		checkSession: async () => undefined,
		onConnectionChange: (connection) => connections.push(connection),
		onMessage: () => {},
	});
	try {
		client.connect();
		await vi.waitFor(() =>
			expect(invalidateToken).toHaveBeenCalledWith("malformed token"),
		);
		expect(connections).toEqual(["connecting", "disconnected"]);
		await vi.advanceTimersByTimeAsync(1_000);
		expect(firstSocket().protocols).toEqual([
			"crypto-scanner.live.v1",
			"renewed-token",
		]);
	} finally {
		client.disconnect();
		vi.useRealTimers();
	}
});
