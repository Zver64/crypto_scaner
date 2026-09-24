import { afterEach, expect, it, vi } from "vitest";
import { LiveCandlesClient } from "@/api/live-candles";

class FakeSocket {
	static OPEN = 1;
	static instances: FakeSocket[] = [];
	readyState = FakeSocket.OPEN;
	messages: unknown[] = [];
	listeners = new Map<string, ((event: { data?: string }) => void)[]>();

	constructor(_url: string) {
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

afterEach(() => {
	vi.unstubAllGlobals();
	FakeSocket.instances = [];
});

it("subscribes to every chart interval on one socket and retains them while switching views", () => {
	vi.stubGlobal("window", {
		location: { href: "https://example.com/instruments/BTC" },
	});
	vi.stubGlobal("WebSocket", FakeSocket);
	const client = new LiveCandlesClient({
		getInitData: () => "signed-data",
		onConnectionChange: () => {},
		onMessage: () => {},
	});
	const subscriptions = (["1h", "1d", "1w", "1M"] as const).map((interval) => ({
		symbol: "BTC",
		interval,
	}));
	client.setSubscriptions(subscriptions);
	client.connect();
	const socket = FakeSocket.instances[0]!;
	socket.emit("open");
	socket.emit("message", JSON.stringify({ type: "authenticated" }));
	expect(socket.messages).toEqual([
		{ type: "authenticate", init_data: "signed-data" },
		...subscriptions.map((subscription) => ({
			type: "subscribe",
			...subscription,
		})),
	]);

	client.setSubscriptions(subscriptions);
	expect(FakeSocket.instances).toHaveLength(1);
	expect(socket.messages).toHaveLength(5);
	client.disconnect();
});

it("updates only changed subscriptions without reopening the socket", () => {
	vi.stubGlobal("window", { location: { href: "https://example.com/" } });
	vi.stubGlobal("WebSocket", FakeSocket);
	const client = new LiveCandlesClient({
		getInitData: () => "signed-data",
		onConnectionChange: () => {},
		onMessage: () => {},
	});
	client.setSubscriptions([{ symbol: "BTC", interval: "1h" }]);
	client.connect();
	const socket = FakeSocket.instances[0]!;
	socket.emit("open");
	socket.emit("message", JSON.stringify({ type: "authenticated" }));
	client.setSubscriptions([{ symbol: "ETH", interval: "1h" }]);
	expect(socket.messages.slice(2)).toEqual([
		{ type: "unsubscribe", symbol: "BTC", interval: "1h" },
		{ type: "subscribe", symbol: "ETH", interval: "1h" },
	]);
	expect(FakeSocket.instances).toHaveLength(1);
	client.disconnect();
});
