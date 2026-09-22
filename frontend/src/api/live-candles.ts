import type {
	CandleInterval,
	LiveCandleClientMessage,
	LiveCandleServerMessage,
} from "@/api/generated/models";

export type LiveCandleConnection = "connecting" | "connected" | "disconnected";

export interface LiveCandleSubscription {
	interval: CandleInterval;
	symbol: string;
}

interface LiveCandlesClientOptions {
	getInitData(): string | undefined;
	onConnectionChange(connection: LiveCandleConnection): void;
	onMessage(message: LiveCandleServerMessage): void;
}

export class LiveCandlesClient {
	private authenticated = false;
	private reconnectAttempt = 0;
	private reconnectTimer: ReturnType<typeof setTimeout> | undefined;
	private socket: WebSocket | undefined;
	private stopped = true;
	private subscription: LiveCandleSubscription | undefined;

	constructor(private readonly options: LiveCandlesClientOptions) {}

	connect(): void {
		if (!this.stopped) return;
		this.stopped = false;
		this.reconnectAttempt = 0;
		this.open();
	}

	setSubscription(subscription: LiveCandleSubscription): void {
		const previous = this.subscription;
		this.subscription = subscription;
		if (!this.authenticated || sameSubscription(previous, subscription)) return;
		if (previous) this.send({ type: "unsubscribe", ...previous });
		this.send({ type: "subscribe", ...subscription });
	}

	disconnect(): void {
		if (this.stopped) return;
		this.stopped = true;
		if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
		this.reconnectTimer = undefined;
		if (this.authenticated && this.subscription) {
			this.send({ type: "unsubscribe", ...this.subscription });
		}
		this.authenticated = false;
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
			if (this.socket !== socket || this.stopped) return;
			const initData = this.options.getInitData()?.trim();
			if (!initData) {
				socket.close(1008, "Telegram authentication is required");
				return;
			}
			this.send({ type: "authenticate", init_data: initData });
		});
		socket.addEventListener("message", (event) => {
			if (this.socket !== socket || this.stopped) return;
			const message = parseLiveMessage(event.data);
			if (!message) return;
			if (message.type === "authenticated") {
				this.authenticated = true;
				this.reconnectAttempt = 0;
				this.options.onConnectionChange("connected");
				if (this.subscription) {
					this.send({ type: "subscribe", ...this.subscription });
				}
				return;
			}
			this.options.onMessage(message);
		});
		socket.addEventListener("close", () => {
			if (this.socket !== socket) return;
			this.socket = undefined;
			this.authenticated = false;
			if (this.stopped) return;
			this.options.onConnectionChange("disconnected");
			const delay = Math.min(30_000, 1_000 * 2 ** this.reconnectAttempt);
			this.reconnectAttempt += 1;
			this.reconnectTimer = setTimeout(() => this.open(), delay);
		});
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
	left: LiveCandleSubscription | undefined,
	right: LiveCandleSubscription,
): boolean {
	return left?.symbol === right.symbol && left.interval === right.interval;
}
