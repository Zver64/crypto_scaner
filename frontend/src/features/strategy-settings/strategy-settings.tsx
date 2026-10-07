import { Button, Group, Loader, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import type { ErrorType } from "@/api/fetch";
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
import type { ErrorResponse, Strategy } from "@/api/generated/models";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";
import { StrategyForm } from "@/features/strategy-settings/strategy-form";
import { StrategyRemovalConfirmation } from "@/features/strategy-settings/strategy-removal-confirmation";
import { StrategyRow } from "@/features/strategy-settings/strategy-row";
import type { StrategyDraft } from "@/features/strategy-settings/types";
import {
	emptyStrategyQuery,
	strategyErrorMessage,
	strategyQuery,
	strategyQueryDropped,
} from "@/features/strategy-settings/utils";

export function StrategySettings() {
	const wide = useWideLayout();
	const queryClient = useQueryClient();
	const [draft, setDraft] = useState<StrategyDraft>();
	const [removing, setRemoving] = useState<Strategy>();
	const strategies = useListStrategies({
		query: { retry: false, select: (response) => response.data.items },
	});
	const variables = useListStrategyVariables({
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
	const failed = (error: ErrorType<ErrorResponse>) => {
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
		mutation: { onError: failed, onSuccess: saved },
	});
	const updateMutation = useUpdateStrategy({
		mutation: { onError: failed, onSuccess: saved },
	});
	const enabledMutation = useSetStrategyEnabled({
		mutation: { onError: failed, onSettled: refresh },
	});
	const deleteMutation = useDeleteStrategy({
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
			{/* Full width on phones, a regular button on wide screens. */}
			<Group grow={!wide}>
				<Button
					disabled={variables.data.length === 0}
					onClick={() =>
						setDraft({
							id: undefined,
							name: "",
							message: "",
							query: emptyStrategyQuery(),
							incomplete: false,
						})
					}
				>
					Add strategy
				</Button>
			</Group>
			{strategies.data.length === 0 ? null : (
				<Stack gap="xs">
					{strategies.data.map((strategy) => (
						<StrategyRow
							disabled={busy}
							key={strategy.id}
							onDelete={() => setRemoving(strategy)}
							onEdit={() => {
								const query = strategyQuery(strategy.expression);
								setDraft({
									id: strategy.id,
									name: strategy.name,
									message: strategy.message,
									query,
									incomplete: strategyQueryDropped(strategy.expression, query),
								});
							}}
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
				onSubmit={(input) => {
					if (draft?.id === undefined) {
						createMutation.mutate({ data: { ...input, enabled: true } });
					} else {
						updateMutation.mutate({
							data: input,
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
