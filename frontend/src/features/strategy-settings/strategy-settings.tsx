import {
	Button,
	Group,
	Loader,
	Paper,
	Stack,
	Text,
	Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
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
import type {
	ErrorResponse,
	Strategy,
	StrategyUpdate,
} from "@/api/generated/models";
import { EmptyState } from "@/components/empty-state";
import { SidebarLayout } from "@/components/sidebar-layout";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";
import { StrategyDiscardConfirmation } from "@/features/strategy-settings/strategy-discard-confirmation";
import { StrategyForm } from "@/features/strategy-settings/strategy-form";
import { StrategyFormContent } from "@/features/strategy-settings/strategy-form-content";
import { StrategyRemovalConfirmation } from "@/features/strategy-settings/strategy-removal-confirmation";
import { StrategyRow } from "@/features/strategy-settings/strategy-row";
import type { StrategyDraft } from "@/features/strategy-settings/types";
import {
	emptyStrategyQuery,
	strategyErrorMessage,
	strategyFormTitle,
	strategyQuery,
	strategyQueryDropped,
} from "@/features/strategy-settings/utils";

export function StrategySettings() {
	const wide = useWideLayout();
	const queryClient = useQueryClient();
	const [draft, setDraft] = useState<StrategyDraft>();
	// Whether the open form differs from its draft.
	const [dirty, setDirty] = useState(false);
	// A strategy waiting to replace an open form with unsaved changes.
	const [pending, setPending] = useState<Omit<StrategyDraft, "revision">>();
	const revision = useRef(0);
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
	const createMutation = useCreateStrategy({
		mutation: { onError: failed, onSuccess: refresh },
	});
	const updateMutation = useUpdateStrategy({
		mutation: { onError: failed, onSuccess: refresh },
	});
	const enabledMutation = useSetStrategyEnabled({
		mutation: { onError: failed, onSettled: refresh },
	});
	const deleteMutation = useDeleteStrategy({
		mutation: {
			onError: failed,
			// A removed strategy leaves the editor beside the list.
			onSuccess: (_, { strategyId }) =>
				setDraft((current) =>
					current?.id === strategyId ? undefined : current,
				),
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
	const isSaving = createMutation.isPending || updateMutation.isPending;
	// Rows wait for saves too, so a strategy is not deleted while it saves.
	const busy =
		enabledMutation.isPending || deleteMutation.isPending || isSaving;
	const show = (next: Omit<StrategyDraft, "revision">) => {
		revision.current += 1;
		setDraft({ ...next, revision: revision.current });
		setDirty(false);
	};
	// Beside the list, opening another strategy would silently drop the
	// changes of the open one, so it asks first.
	const open = (next: Omit<StrategyDraft, "revision">) => {
		if (draft && dirty) setPending(next);
		else show(next);
	};
	const submit = (input: StrategyUpdate) => {
		if (!draft) return;
		// A finished save closes the form it came from, not one opened since.
		const saved = draft.revision;
		const close = () =>
			setDraft((current) =>
				current?.revision === saved ? undefined : current,
			);
		if (draft.id === undefined) {
			createMutation.mutate(
				{ data: { ...input, enabled: true } },
				{ onSuccess: close },
			);
		} else {
			updateMutation.mutate(
				{ data: input, strategyId: draft.id },
				{ onSuccess: close },
			);
		}
	};
	const list = (
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
			{/* Full width on phones and in the sidebar. */}
			<Group grow>
				<Button
					disabled={variables.data.length === 0}
					onClick={() =>
						open({
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
								open({
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
							selected={wide && draft?.id === strategy.id}
							strategy={strategy}
						/>
					))}
				</Stack>
			)}
		</Stack>
	);
	const confirmations = (
		<>
			<StrategyDiscardConfirmation
				onCancel={() => setPending(undefined)}
				onConfirm={() => {
					if (pending) show(pending);
					setPending(undefined);
				}}
				opened={pending !== undefined}
			/>
			<StrategyRemovalConfirmation
				isPending={deleteMutation.isPending}
				name={removing?.name}
				onCancel={() => setRemoving(undefined)}
				onConfirm={() => {
					if (removing) deleteMutation.mutate({ strategyId: removing.id });
				}}
			/>
		</>
	);
	// Phones edit a strategy in a full-screen form over the list; wide
	// screens keep the list in the sidebar and edit beside it.
	if (!wide) {
		return (
			<>
				{list}
				<StrategyForm
					draft={draft}
					isSaving={isSaving}
					onCancel={() => setDraft(undefined)}
					onDirtyChange={setDirty}
					onSubmit={submit}
					variables={variables.data}
				/>
				{confirmations}
			</>
		);
	}
	return (
		<SidebarLayout gap="md" sidebar={list} sidebarPosition="start">
			{draft ? (
				<Paper p="md" radius="sm" withBorder>
					<Stack gap="md">
						<Title order={2} size="h4">
							{strategyFormTitle(draft)}
						</Title>
						<StrategyFormContent
							draft={draft}
							isSaving={isSaving}
							key={draft.revision}
							onCancel={() => setDraft(undefined)}
							onDirtyChange={setDirty}
							onSubmit={submit}
							variables={variables.data}
						/>
					</Stack>
				</Paper>
			) : (
				<EmptyState
					description="Choose a strategy to edit, or add one."
					title="No strategy open"
				/>
			)}
			{confirmations}
		</SidebarLayout>
	);
}
