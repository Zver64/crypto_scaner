import { Button, Group, Loader, Paper, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import type { ErrorType } from "@/api/fetch";
import {
	getListScannerIndicatorsQueryKey,
	getListStrategiesQueryKey,
	useCreateScannerIndicatorBatch,
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
import { invalidateScannerIndicatorQueries } from "@/features/scanner-settings/query-cache";
import { mutationErrorMessage } from "@/features/scanner-settings/utils";
import { kindNouns } from "@/features/strategy-settings/constants";
import { StrategyDiscardConfirmation } from "@/features/strategy-settings/strategy-discard-confirmation";
import { StrategyForm } from "@/features/strategy-settings/strategy-form";
import { StrategyFormContent } from "@/features/strategy-settings/strategy-form-content";
import { StrategyIndicatorsConfirmation } from "@/features/strategy-settings/strategy-indicators-confirmation";
import { StrategyRemovalConfirmation } from "@/features/strategy-settings/strategy-removal-confirmation";
import { StrategyRow } from "@/features/strategy-settings/strategy-row";
import type {
	StrategyDraft,
	StrategyKind,
} from "@/features/strategy-settings/types";
import {
	newStrategyDraft,
	strategyDraft,
	strategyErrorMessage,
	strategyKind,
} from "@/features/strategy-settings/utils";

interface StrategySettingsProps {
	// Whether the page lists the strategies or the signals; each page creates
	// and edits only its own kind.
	kind: StrategyKind;
}

export function StrategySettings({ kind }: StrategySettingsProps) {
	const nouns = kindNouns[kind];
	const wide = useWideLayout();
	const queryClient = useQueryClient();
	const [draft, setDraft] = useState<StrategyDraft>();
	// Whether the open form differs from its draft.
	const [dirty, setDirty] = useState(false);
	// A strategy waiting to replace an open form with unsaved changes.
	const [pending, setPending] = useState<Omit<StrategyDraft, "revision">>();
	const revision = useRef(0);
	const [removing, setRemoving] = useState<Strategy>();
	// A strategy to open once the indicators it reads are added.
	const [adding, setAdding] = useState<Strategy>();
	const strategies = useListStrategies({
		query: {
			retry: false,
			select: (response) =>
				response.data.items.filter(
					(strategy) => strategyKind(strategy) === kind,
				),
		},
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
			title: `${nouns.name} change failed`,
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

	// The builder shows the added indicators once the variables are current.
	const additionMutation = useCreateScannerIndicatorBatch({
		mutation: {
			onError: (error) => {
				setAdding(undefined);
				notifications.show({
					color: "red",
					message: mutationErrorMessage(error),
					title: "Indicators could not be added",
				});
			},
			onSuccess: () => invalidateScannerIndicatorQueries(queryClient),
		},
	});

	if (strategies.isPending || variables.isPending) {
		return <Loader aria-label={`Loading ${nouns.many}`} />;
	}
	if (strategies.isError || variables.isError) {
		return (
			<Text c="red" size="sm">
				{`${nouns.title} could not be loaded.`}
			</Text>
		);
	}
	const isSaving = createMutation.isPending || updateMutation.isPending;
	// Rows wait for saves too, so a strategy is not deleted while it saves.
	const busy =
		enabledMutation.isPending ||
		deleteMutation.isPending ||
		additionMutation.isPending ||
		isSaving;
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
					{`No ${nouns.many} yet.`}
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
					onClick={() => open(newStrategyDraft(kind))}
				>
					{`Add ${nouns.one}`}
				</Button>
			</Group>
			{strategies.data.length === 0 ? null : (
				<Stack gap="xs">
					{strategies.data.map((strategy) => (
						<StrategyRow
							disabled={busy}
							key={strategy.id}
							onDelete={() => setRemoving(strategy)}
							onEdit={() =>
								strategy.missing_indicators.length > 0
									? setAdding(strategy)
									: open(strategyDraft(strategy))
							}
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
			<StrategyIndicatorsConfirmation
				isPending={additionMutation.isPending}
				missing={adding?.missing_indicators}
				onCancel={() => setAdding(undefined)}
				onConfirm={() => {
					if (!adding) return;
					additionMutation.mutate(
						{
							data: {
								items: adding.missing_indicators.map(
									({ interval, parameters, type }) => ({
										interval,
										parameters,
										type,
									}),
								),
							},
						},
						{
							onSuccess: () => {
								open(strategyDraft(adding));
								setAdding(undefined);
							},
						},
					);
				}}
			/>
			<StrategyRemovalConfirmation
				isPending={deleteMutation.isPending}
				name={removing?.name}
				noun={nouns.one}
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
					description={`Choose a ${nouns.one} to edit, or add one.`}
					fillHeight
					title={`No ${nouns.one} open`}
				/>
			)}
			{confirmations}
		</SidebarLayout>
	);
}
