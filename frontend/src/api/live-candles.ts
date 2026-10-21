import type {
	CandleInterval,
	IndicatorConfig,
	LiveCandleClientMessage,
	LiveCandleServerMessage,
} from "@/api/generated/models";

// The backend authenticates the WebSocket handshake by the session token
// offered after this subprotocol, and selects only this one.
const liveCandleProtocol = "crypto-scanner.live.v1";

export type LiveCandleConnection = "connecting" | "connected" | "disconnected";

export interface LiveCandleSubscription {
	interval: CandleInterval;
	symbol: string;
	limit?: number;
	indicators?: IndicatorConfig[];
}

interface LiveCandlesClientOptions {
	getToken(): Promise<string | undefined>;
	// Forgets a token the server rejected, so the reconnect gets a new one.
	invalidateToken(token: string): void;
	// Sends an authenticated request, which renews a rejected session. The
	// browser hides why a handshake failed, so a socket that closes before it
	// opens checks the session this way before reconnecting.
	checkSession(): Promise<unknown>;
	onConnectionChange(connection: LiveCandleConnection): void;
	onMessage(message: LiveCandleServerMessage): void;
}

export class LiveCandlesClient {
	// Counts open() calls, so a token resolved for a stale one is dropped.
	private generation = 0;
	// The socket is open: the handshake authenticated it.
	private ready = false;
	private reconnectAttempt = 0;
	private reconnectTimer: ReturnType<typeof setTimeout> | undefined;
	private socket: WebSocket | undefined;
	private stopped = true;
	private subscriptions: readonly LiveCandleSubscription[] = [];

	constructor(private readonly options: LiveCandlesClientOptions) {}

	connect(): void {
		if (!this.stopped) return;
		this.stopped = false;
		this.reconnectAttempt = 0;
		void this.open();
	}

	setSubscriptions(subscriptions: readonly LiveCandleSubscription[]): void {
		const previous = this.subscriptions;
		this.subscriptions = subscriptions;
		if (!this.ready) return;
		for (const subscription of previous) {
			if (!subscriptions.some((next) => sameSubscription(subscription, next))) {
				this.send({ type: "unsubscribe", ...subscription });
			}
		}
		for (const subscription of subscriptions) {
			if (
				!previous.some(
					(old) =>
						sameSubscription(old, subscription) &&
						old.limit === subscription.limit &&
						JSON.stringify(old.indicators) ===
							JSON.stringify(subscription.indicators),
				)
			) {
				this.send({ type: "subscribe", ...subscription });
			}
		}
	}

	// Subscribing again to a subscribed chart makes the backend send a new
	// snapshot of it.
	resync(symbol: string, interval: CandleInterval): void {
		if (!this.ready) return;
		const subscription = this.subscriptions.find(
			(item) => item.symbol === symbol && item.interval === interval,
		);
		if (subscription) this.send({ type: "subscribe", ...subscription });
	}

	disconnect(): void {
		if (this.stopped) return;
		this.stopped = true;
		this.generation += 1;
		if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
		this.reconnectTimer = undefined;
		if (this.ready) {
			for (const subscription of this.subscriptions) {
				this.send({ type: "unsubscribe", ...subscription });
			}
		}
		this.ready = false;
		const socket = this.socket;
		this.socket = undefined;
		socket?.close(1000, "Page closed");
	}

	private async open(): Promise<void> {
		if (this.stopped) return;
		const generation = ++this.generation;
		this.options.onConnectionChange("connecting");
		const token = await this.options.getToken().catch(() => undefined);
		if (generation !== this.generation || this.stopped) return;
		if (!token) {
			this.reconnect();
			return;
		}
		let socket: WebSocket;
		try {
			socket = new WebSocket(liveWebSocketURL(), [liveCandleProtocol, token]);
		} catch {
			// A token that is not a valid subprotocol can never authenticate.
			this.options.invalidateToken(token);
			this.handshakeFailed(generation);
			return;
		}
		this.socket = socket;
		socket.addEventListener("open", () => {
			if (this.socket !== socket || this.stopped) return;
			this.ready = true;
			this.reconnectAttempt = 0;
			this.options.onConnectionChange("connected");
			for (const subscription of this.subscriptions) {
				this.send({ type: "subscribe", ...subscription });
			}
		});
		socket.addEventListener("message", (event) => {
			if (this.socket !== socket || this.stopped) return;
			const message = parseLiveMessage(event.data);
			if (!message) return;
			if (message.type === "error" && message.code === "unauthenticated") {
				this.options.invalidateToken(token);
			}
			this.options.onMessage(message);
		});
		socket.addEventListener("close", () => {
			if (this.socket !== socket) return;
			const opened = this.ready;
			this.socket = undefined;
			this.ready = false;
			if (this.stopped) return;
			if (opened) {
				this.reconnect();
				return;
			}
			this.handshakeFailed(generation);
		});
	}

	// A failed handshake keeps growing the reconnect delay: only an opened
	// socket resets it.
	private handshakeFailed(generation: number): void {
		this.options.onConnectionChange("disconnected");
		void this.options
			.checkSession()
			.catch(() => undefined)
			.then(() => {
				if (generation === this.generation && !this.stopped) {
					this.scheduleOpen();
				}
			});
	}

	private reconnect(): void {
		this.options.onConnectionChange("disconnected");
		this.scheduleOpen();
	}

	private scheduleOpen(): void {
		const delay = Math.min(30_000, 1_000 * 2 ** this.reconnectAttempt);
		this.reconnectAttempt += 1;
		this.reconnectTimer = setTimeout(() => void this.open(), delay);
	}

	private send(message: LiveCandleClientMessage): void {
		if (this.socket?.readyState === WebSocket.OPEN) {
			this.socket.send(JSON.stringify(message));
		}
	}
}

function liveWebSocketURL(): string {
	const url = new URL("/api/v1/live/candles", window.location.href);
	url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
	return url.toString();
}

function parseLiveMessage(value: unknown): LiveCandleServerMessage | undefined {
	if (typeof value !== "string") return undefined;
	try {
		const parsed = JSON.parse(value) as Partial<LiveCandleServerMessage>;
		return typeof parsed.type === "string"
			? (parsed as LiveCandleServerMessage)
			: undefined;
	} catch {
		return undefined;
	}
}

function sameSubscription(
	left: LiveCandleSubscription,
	right: LiveCandleSubscription,
): boolean {
	return left?.symbol === right.symbol && left.interval === right.interval;
}
