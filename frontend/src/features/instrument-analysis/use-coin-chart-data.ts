import { useEffect, useState } from "react";

import {
	type CoinChartData,
	createCoinChartData,
} from "@/features/instrument-analysis/coin-chart-data";
import type { ChartCatalogs } from "@/features/instrument-analysis/live-candle-store";

// The live candles of the coin page, shared by the chart and the price alerts.
// The page streams them while enabled, whether or not the chart is shown. The
// source owns a WebSocket subscription, so it is state replaced only when the
// symbol changes, never a value recomputed at React's choice.
export function useCoinChartData(
	symbol: string,
	catalogs: ChartCatalogs | undefined,
	enabled: boolean,
): CoinChartData {
	const [state, setState] = useState(() => coinChartState(symbol));
	const current = state.symbol === symbol ? state : coinChartState(symbol);
	if (current !== state) setState(current);
	const { source } = current;
	// Declared before the stream starts, so cached catalogs are in its first
	// subscription.
	useEffect(() => {
		if (catalogs) source.setCatalogs(catalogs);
	}, [source, catalogs]);
	useEffect(() => {
		if (!enabled) return;
		source.start();
		return () => source.stop();
	}, [enabled, source]);
	return source;
}

function coinChartState(symbol: string) {
	return { symbol, source: createCoinChartData(symbol) };
}
