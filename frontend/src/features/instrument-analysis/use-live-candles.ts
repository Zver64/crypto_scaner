import {
	type Dispatch,
	type RefObject,
	type SetStateAction,
	useEffect,
	useRef,
	useState,
} from "react";
import type {
	CandleInterval,
	LiveCandleServerMessage,
	LiveCandleServerMessageFreshness,
	LiveCandleState,
} from "@/api/generated/models";
import {
	type LiveCandleSubscription,
	LiveCandlesClient,
} from "@/api/live-candles";
import { getTelegramInitData } from "@/app/telegram";
import { validateCandles } from "@/features/instrument-analysis/candle-page";
import { applyLiveCandleUpdate } from "@/features/instrument-analysis/live-candle-merge";

export interface LiveCandlesState {
	candles: readonly LiveCandleState[];
	connection: "connecting" | "connected" | "disconnected";
	freshness: LiveCandleServerMessageFreshness;
	error?: string;
}

const initialState: LiveCandlesState = {
	candles: [],
	connection: "connecting",
	freshness: "waiting",
};

export function useLiveCandles(
	enabled: boolean,
	symbol: string,
	interval: CandleInterval,
): LiveCandlesState {
	const [state, setState] = useState<LiveCandlesState>(initialState);
	const selectionRef = useRef<LiveCandleSubscription>({
		interval,
		symbol: symbol.toUpperCase(),
	});
	const clientRef = useRef<LiveCandlesClient | undefined>(undefined);

	useEffect(() => {
		const next = { interval, symbol: symbol.toUpperCase() };
		selectionRef.current = next;
		setState((current) => ({
			...current,
			candles: [],
			freshness: "waiting",
			error: undefined,
		}));
		clientRef.current?.setSubscription(next);
	}, [interval, symbol]);

	useEffect(() => {
		if (!enabled) {
			setState({ ...initialState, connection: "disconnected" });
			return;
		}
		setState(initialState);
		const client = new LiveCandlesClient({
			getInitData: getTelegramInitData,
			onConnectionChange: (connection) => {
				setState((current) => ({
					...current,
					connection,
					// An error from the previous socket must not remain visible while
					// a new connection is authenticating or loading its snapshot.
					error: connection === "disconnected" ? current.error : undefined,
					freshness:
						connection === "connected"
							? current.freshness
							: current.candles.length > 0
								? "stale"
								: "waiting",
				}));
			},
			onMessage: (message) =>
				applyServerMessage(message, selectionRef, setState),
		});
		clientRef.current = client;
		client.setSubscription(selectionRef.current);
		client.connect();
		return () => {
			if (clientRef.current === client) clientRef.current = undefined;
			client.disconnect();
		};
	}, [enabled]);

	const currentSelection = selectionRef.current;
	if (
		currentSelection.symbol !== symbol.toUpperCase() ||
		currentSelection.interval !== interval
	) {
		return {
			...state,
			candles: [],
			error: undefined,
			freshness: "waiting",
		};
	}
	return state;
}

function applyServerMessage(
	message: LiveCandleServerMessage,
	selectionRef: RefObject<LiveCandleSubscription>,
	setState: Dispatch<SetStateAction<LiveCandlesState>>,
): void {
	const expected = selectionRef.current;
	if (message.type === "error") {
		if (
			(message.symbol === undefined || message.symbol === expected.symbol) &&
			(message.interval === undefined || message.interval === expected.interval)
		) {
			setState((current) => ({
				...current,
				error: message.message ?? "Live candles are unavailable",
				freshness: "stale",
			}));
		}
		return;
	}
	if (
		message.symbol !== expected.symbol ||
		message.interval !== expected.interval
	) {
		return;
	}
	if (message.type === "snapshot" && message.candles) {
		const candles = message.candles;
		setState((current) => ({
			...current,
			candles: validLiveCandles(candles) ? candles : current.candles,
			freshness: message.freshness ?? current.freshness,
			error: undefined,
		}));
	} else if (message.type === "update" && message.candle) {
		if (!validLiveCandles([message.candle])) return;
		setState((current) => ({
			...current,
			candles: applyLiveCandleUpdate(current.candles, message.candle!),
			freshness: message.freshness ?? "fresh",
			error: undefined,
		}));
	} else if (message.type === "status" && message.freshness) {
		setState((current) => ({
			...current,
			freshness: message.freshness!,
		}));
	}
}

function validLiveCandles(candles: readonly LiveCandleState[]) {
	try {
		validateCandles(
			[...candles]
				.map((state) => state.candle)
				.sort(
					(left, right) =>
						Date.parse(left.open_time) - Date.parse(right.open_time),
				),
		);
		return candles.every((state) => typeof state.final === "boolean");
	} catch {
		return false;
	}
}
