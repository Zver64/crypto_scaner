import { useState } from "react";

import {
	type CoinChartData,
	createCoinChartData,
} from "@/features/instrument-analysis/coin-chart-data";
import type { ChartCatalogs } from "@/features/instrument-analysis/live-candle-store";

interface CoinChartState {
	symbol: string;
	catalogs?: ChartCatalogs;
	source?: CoinChartData;
}

// The live chart source of a symbol, shared by the chart and the price alerts.
// It owns a WebSocket subscription, so it is state replaced only when the
// symbol or the catalogs change, never a value recomputed at React's choice.
export function useCoinChartData(
	symbol: string,
	catalogs: ChartCatalogs | undefined,
): CoinChartData | undefined {
	const [state, setState] = useState(() => coinChartState(symbol, catalogs));
	if (state.symbol === symbol && state.catalogs === catalogs) {
		return state.source;
	}
	const next = coinChartState(symbol, catalogs);
	setState(next);
	return next.source;
}

function coinChartState(
	symbol: string,
	catalogs: ChartCatalogs | undefined,
): CoinChartState {
	return {
		symbol,
		catalogs,
		source: catalogs && createCoinChartData(symbol, catalogs),
	};
}
