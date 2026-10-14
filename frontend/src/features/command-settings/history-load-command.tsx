import {
	Button,
	Group,
	Loader,
	Paper,
	Stack,
	Text,
	Title,
} from "@mantine/core";
import { useSelector } from "@tanstack/react-form";
import { useQueryClient } from "@tanstack/react-query";

import {
	type getCandleHistoryLoadResponseSuccess,
	getGetCandleHistoryLoadQueryKey,
	useStartCandleHistoryLoad,
} from "@/api/generated/api";
import { useAppForm } from "@/components/form/use-app-form";
import { chartIntervalOptions } from "@/features/candle-chart/config";
import { historyLoadDepth } from "@/features/command-settings/constants";
import { HistoryLoadCoinsSelect } from "@/features/command-settings/history-load-coins-select";
import { HistoryLoadJob } from "@/features/command-settings/history-load-job";
import type { HistoryLoadValues } from "@/features/command-settings/types";
import { useHistoryLoadJob } from "@/features/command-settings/use-history-load-job";
import {
	historyLoadErrorMessage,
	historyLoadRequest,
} from "@/features/command-settings/utils";
import { formatNumber } from "@/utils/number-format";

const defaultValues: HistoryLoadValues = {
	coins: [],
	depth: historyLoadDepth.default,
	intervals: ["1h"],
};

// Starts a deeper history load and follows the latest one, which the server
// keeps in memory until it restarts.
export function HistoryLoadCommand() {
	const queryClient = useQueryClient();
	const form = useAppForm({ defaultValues });
	// The Start button follows the values as they change.
	const request = historyLoadRequest(
		useSelector(form.store, (state) => state.values),
	);
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
				<form.Field name="coins">
					{(field) => (
						<HistoryLoadCoinsSelect
							disabled={running}
							onBlur={field.handleBlur}
							onChange={field.handleChange}
							value={field.state.value}
						/>
					)}
				</form.Field>
				<form.AppField name="intervals">
					{(field) => (
						<field.MultiSelectField
							data={chartIntervalOptions.map(({ label, value }) => ({
								label,
								value,
							}))}
							disabled={running}
							label="Intervals"
						/>
					)}
				</form.AppField>
				<form.AppField name="depth">
					{(field) => (
						<field.NumberField
							allowDecimal={false}
							description={`Closed candles to keep per coin and interval, ${formatNumber(historyLoadDepth.min)}–${formatNumber(historyLoadDepth.max)}.`}
							disabled={running}
							label="Depth"
							max={historyLoadDepth.max}
							min={historyLoadDepth.min}
							thousandSeparator=","
						/>
					)}
				</form.AppField>
				<Group justify="flex-end">
					<Button
						disabled={running || !request}
						loading={start.isPending}
						onClick={() => {
							if (request) start.mutate({ data: request });
						}}
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
