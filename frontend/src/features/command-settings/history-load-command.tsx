import {
	Button,
	Group,
	Loader,
	MultiSelect,
	NumberInput,
	Paper,
	Stack,
	Text,
	Title,
} from "@mantine/core";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
	type getCandleHistoryLoadResponseSuccess,
	getGetCandleHistoryLoadQueryKey,
	useStartCandleHistoryLoad,
} from "@/api/generated/api";
import type { CandleInterval } from "@/api/generated/models";
import { chartIntervalOptions } from "@/features/candle-chart/config";
import { historyLoadDepth } from "@/features/command-settings/constants";
import { HistoryLoadCoinsSelect } from "@/features/command-settings/history-load-coins-select";
import { HistoryLoadJob } from "@/features/command-settings/history-load-job";
import { useHistoryLoadJob } from "@/features/command-settings/use-history-load-job";
import { historyLoadErrorMessage } from "@/features/command-settings/utils";
import { formatNumber } from "@/utils/number-format";

// Starts a deeper history load and follows the latest one, which the server
// keeps in memory until it restarts.
export function HistoryLoadCommand() {
	const queryClient = useQueryClient();
	const [coins, setCoins] = useState<string[]>([]);
	const [intervals, setIntervals] = useState<CandleInterval[]>(["1h"]);
	const [depth, setDepth] = useState<number>(historyLoadDepth.default);
	const job = useHistoryLoadJob();
	const start = useStartCandleHistoryLoad({
		mutation: {
			onSuccess: async (response) => {
				// An older GET must not overwrite the accepted job, even if the
				// next GET fails.
				const queryKey = getGetCandleHistoryLoadQueryKey();
				await queryClient.cancelQueries({ queryKey });
				queryClient.setQueryData<getCandleHistoryLoadResponseSuccess>(
					queryKey,
					{
						...response,
						status: 200,
					},
				);
			},
			onSettled: () =>
				queryClient.invalidateQueries({
					queryKey: getGetCandleHistoryLoadQueryKey(),
				}),
		},
	});
	const running = job.data?.status === "running";
	const validDepth =
		depth >= historyLoadDepth.min && depth <= historyLoadDepth.max;
	return (
		<Paper p="md">
			<Stack gap="sm">
				<Title order={3} size="h5">
					Load deeper candle history
				</Title>
				<Text c="dimmed" size="sm">
					Downloads older candles from Binance in the background, about one page
					of 1,000 candles per second, so backtests can replay more history. It
					runs only when started here.
				</Text>
				<HistoryLoadCoinsSelect
					disabled={running}
					onChange={setCoins}
					value={coins}
				/>
				<MultiSelect
					data={chartIntervalOptions.map(({ label, value }) => ({
						label,
						value,
					}))}
					disabled={running}
					label="Intervals"
					onChange={(values) =>
						setIntervals(
							chartIntervalOptions
								.map(({ value }) => value)
								.filter((value) => values.includes(value)),
						)
					}
					value={intervals}
				/>
				<NumberInput
					allowDecimal={false}
					clampBehavior="strict"
					description={`Closed candles to keep per coin and interval, ${formatNumber(historyLoadDepth.min)}–${formatNumber(historyLoadDepth.max)}.`}
					disabled={running}
					label="Depth"
					max={historyLoadDepth.max}
					min={historyLoadDepth.min}
					onChange={(value) => setDepth(Number(value))}
					thousandSeparator=","
					value={depth}
				/>
				<Group justify="flex-end">
					<Button
						disabled={
							running ||
							coins.length === 0 ||
							intervals.length === 0 ||
							!validDepth
						}
						loading={start.isPending}
						onClick={() =>
							start.mutate({
								data: { depth, intervals, symbols: coins },
							})
						}
					>
						Start
					</Button>
				</Group>
				{start.isError ? (
					<Text c="red" size="sm">
						{historyLoadErrorMessage(start.error)}
					</Text>
				) : null}
				{job.isPending ? (
					<Loader size="sm" />
				) : job.isError ? (
					<Text c="red" size="sm">
						The latest history load could not be read.
					</Text>
				) : job.data ? (
					<HistoryLoadJob job={job.data} />
				) : (
					<Text c="dimmed" size="sm">
						No history load has run since the server started.
					</Text>
				)}
			</Stack>
		</Paper>
	);
}
