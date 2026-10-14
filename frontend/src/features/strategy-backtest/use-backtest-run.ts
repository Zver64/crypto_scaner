import { useEffect, useRef, useState } from "react";

import { backtestStrategy } from "@/api/generated/api";
import { runBacktests } from "@/features/strategy-backtest/run-backtests";
import type {
	BacktestRun,
	BacktestRunParameters,
} from "@/features/strategy-backtest/types";
import { backtestPeriod } from "@/features/strategy-backtest/utils";

export function useBacktestRun() {
	const [run, setRun] = useState<BacktestRun>();
	const [running, setRunning] = useState(false);
	const active = useRef<AbortController | undefined>(undefined);
	const revision = useRef(0);
	useEffect(
		() => () => {
			active.current?.abort();
			active.current = undefined;
		},
		[],
	);

	async function start(parameters: BacktestRunParameters) {
		if (active.current) return;
		const controller = new AbortController();
		active.current = controller;
		const snapshot = { ...parameters, symbols: [...parameters.symbols] };
		const currentRevision = ++revision.current;
		const startedAt = Date.now();
		setRunning(true);
		try {
			await runBacktests(
				snapshot,
				controller.signal,
				async (symbol, chart, signal) => {
					const response = await backtestStrategy(
						snapshot.strategy,
						{ symbol, chart, ...backtestPeriod(snapshot.from, snapshot.to) },
						{ signal },
					);
					return response.data;
				},
				(items) => {
					if (active.current === controller) {
						setRun({
							parameters: snapshot,
							items,
							revision: currentRevision,
							startedAt,
						});
					}
				},
			);
		} finally {
			if (active.current === controller) {
				active.current = undefined;
				setRunning(false);
			}
		}
	}
	return { run, running, start, cancel: () => active.current?.abort() };
}
