import type {
	CandleInterval,
	IndicatorConfig,
	LiveCandleClientMessage,
	LiveCandleServerMessage,
} from "@/api/generated/models";

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
	onConnectionChange(connection: LiveCandleConnection): void;
	onMessage(message: LiveCandleServerMessage): void;
}

export class LiveCandlesClient {
	// The socket sent authenticate. The server reads messages in order, so
	// subscriptions follow it at once instead of waiting for authenticated.
	private ready = false;
	private reconnectAttempt = 0;
	private reconnectTimer: ReturnType<typeof setTimeout> | undefined;
	private socket: WebSocket | undefined;
	private stopped = true;
	private token: string | undefined;
	private subscriptions: readonly LiveCandleSubscription[] = [];

	constructor(private readonly options: LiveCandlesClientOptions) {}

	connect(): void {
		if (!this.stopped) return;
		this.stopped = false;
		this.reconnectAttempt = 0;
		this.open();
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

	disconnect(): void {
		if (this.stopped) return;
		this.stopped = true;
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

	private open(): void {
		if (this.stopped) return;
		this.options.onConnectionChange("connecting");
		const socket = new WebSocket(liveWebSocketURL());
		this.socket = socket;
		socket.addEventListener("open", () => {
			void this.authenticate(socket);
		});
		socket.addEventListener("message", (event) => {
			if (this.socket !== socket || this.stopped) return;
			const message = parseLiveMessage(event.data);
			if (!message) return;
			if (message.type === "authenticated") {
				this.reconnectAttempt = 0;
				this.options.onConnectionChange("connected");
				return;
			}
			if (
				message.type === "error" &&
				message.code === "unauthenticated" &&
				this.token
			) {
				this.options.invalidateToken(this.token);
			}
			this.options.onMessage(message);
		});
		socket.addEventListener("close", () => {
			if (this.socket !== socket) return;
			this.socket = undefined;
			this.ready = false;
			if (this.stopped) return;
			this.options.onConnectionChange("disconnected");
			const delay = Math.min(30_000, 1_000 * 2 ** this.reconnectAttempt);
			this.reconnectAttempt += 1;
			this.reconnectTimer = setTimeout(() => this.open(), delay);
		});
	}

	private async authenticate(socket: WebSocket): Promise<void> {
		let token: string | undefined;
		try {
			token = await this.options.getToken();
		} catch {
			token = undefined;
		}
		if (this.socket !== socket || this.stopped) return;
		if (!token) {
			socket.close(1008, "Authentication is required");
			return;
		}
		this.token = token;
		this.send({ type: "authenticate", token });
		this.ready = true;
		for (const subscription of this.subscriptions) {
			this.send({ type: "subscribe", ...subscription });
		}
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
