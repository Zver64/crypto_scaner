import { Button, Loader, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
	getListScannerIndicatorsQueryKey,
	getListStrategiesQueryKey,
	useCreateStrategy,
	useDeleteStrategy,
	useListStrategies,
	useListStrategyVariables,
	useSetStrategyEnabled,
	useUpdateStrategy,
} from "@/api/generated/api";
import type { Strategy } from "@/api/generated/models";
import { telegramRequestOptions } from "@/app/telegram";
import { StrategyForm } from "@/features/strategy-settings/strategy-form";
import { StrategyRemovalConfirmation } from "@/features/strategy-settings/strategy-removal-confirmation";
import { StrategyRow } from "@/features/strategy-settings/strategy-row";
import type { StrategyDraft } from "@/features/strategy-settings/types";
import {
	emptyStrategyQuery,
	strategyErrorMessage,
	strategyQuery,
} from "@/features/strategy-settings/utils";

export function StrategySettings() {
	const queryClient = useQueryClient();
	const [draft, setDraft] = useState<StrategyDraft>();
	const [removing, setRemoving] = useState<Strategy>();
	const strategies = useListStrategies({
		fetch: telegramRequestOptions(),
		query: { retry: false, select: (response) => response.data.items },
	});
	const variables = useListStrategyVariables({
		fetch: telegramRequestOptions(),
		query: { retry: false, select: (response) => response.data.items },
	});
	// Strategies decide which indicators are in use.
	const refresh = () => {
		void queryClient.invalidateQueries({
			queryKey: getListScannerIndicatorsQueryKey(),
		});
		return queryClient.invalidateQueries({
			queryKey: getListStrategiesQueryKey(),
		});
	};
	const failed = (error: unknown) => {
		notifications.show({
			color: "red",
			message: strategyErrorMessage(error),
			title: "Strategy change failed",
		});
	};
	const saved = () => {
		setDraft(undefined);
		return refresh();
	};
	const createMutation = useCreateStrategy({
		fetch: telegramRequestOptions(),
		mutation: { onError: failed, onSuccess: saved },
	});
	const updateMutation = useUpdateStrategy({
		fetch: telegramRequestOptions(),
		mutation: { onError: failed, onSuccess: saved },
	});
	const enabledMutation = useSetStrategyEnabled({
		fetch: telegramRequestOptions(),
		mutation: { onError: failed, onSettled: refresh },
	});
	const deleteMutation = useDeleteStrategy({
		fetch: telegramRequestOptions(),
		mutation: {
			onError: failed,
			onSettled: () => {
				setRemoving(undefined);
				return refresh();
			},
		},
	});

	if (strategies.isPending || variables.isPending) {
		return <Loader aria-label="Loading strategies" />;
	}
	if (strategies.isError || variables.isError) {
		return (
			<Text c="red" size="sm">
				Strategies could not be loaded.
			</Text>
		);
	}
	const busy = enabledMutation.isPending || deleteMutation.isPending;
	return (
		<Stack gap="md">
			{strategies.data.length === 0 ? (
				<Text c="dimmed" size="sm">
					No strategies yet.
				</Text>
			) : null}
			{variables.data.length === 0 ? (
				<Text c="dimmed" size="sm">
					Add indicators first.
				</Text>
			) : null}
			<Button
				disabled={variables.data.length === 0}
				fullWidth
				onClick={() =>
					setDraft({ id: undefined, name: "", query: emptyStrategyQuery() })
				}
			>
				Add strategy
			</Button>
			{strategies.data.length === 0 ? null : (
				<Stack gap="xs">
					{strategies.data.map((strategy) => (
						<StrategyRow
							disabled={busy}
							key={strategy.id}
							onDelete={() => setRemoving(strategy)}
							onEdit={() =>
								setDraft({
									id: strategy.id,
									name: strategy.name,
									query: strategyQuery(strategy.expression),
								})
							}
							onEnabledChange={(enabled) =>
								enabledMutation.mutate({
									data: { enabled },
									strategyId: strategy.id,
								})
							}
							strategy={strategy}
						/>
					))}
				</Stack>
			)}
			<StrategyForm
				draft={draft}
				isSaving={createMutation.isPending || updateMutation.isPending}
				onCancel={() => setDraft(undefined)}
				onSubmit={(name, expression) => {
					if (draft?.id === undefined) {
						createMutation.mutate({
							data: { enabled: true, expression, name },
						});
					} else {
						updateMutation.mutate({
							data: { expression, name },
							strategyId: draft.id,
						});
					}
				}}
				variables={variables.data}
			/>
			<StrategyRemovalConfirmation
				isPending={deleteMutation.isPending}
				name={removing?.name}
				onCancel={() => setRemoving(undefined)}
				onConfirm={() => {
					if (removing) deleteMutation.mutate({ strategyId: removing.id });
				}}
			/>
		</Stack>
	);
}
