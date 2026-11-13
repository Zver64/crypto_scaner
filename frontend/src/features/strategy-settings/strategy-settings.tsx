import { Button, Group, Loader, Paper, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";

import type { ErrorType } from "@/api/fetch";
import {
	getListScannerIndicatorsQueryKey,
	getListStrategiesQueryKey,
	type getStrategyCatalogResponseSuccess,
	type listStrategiesResponseSuccess,
	useCreateStrategy,
	useDeleteStrategy,
	useGetStrategyCatalog,
	useListStrategies,
	useSetStrategyEnabled,
	useUpdateStrategy,
} from "@/api/generated/api";
import type { ErrorResponse, Strategy } from "@/api/generated/models";
import { EmptyState } from "@/components/empty-state";
import { SidebarLayout } from "@/components/sidebar-layout";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";
import { kindNouns } from "@/features/strategy-settings/constants";
import { StrategyDiscardConfirmation } from "@/features/strategy-settings/strategy-discard-confirmation";
import { StrategyForm } from "@/features/strategy-settings/strategy-form";
import { StrategyFormContent } from "@/features/strategy-settings/strategy-form-content";
import { StrategyRemovalConfirmation } from "@/features/strategy-settings/strategy-removal-confirmation";
import { StrategyRow } from "@/features/strategy-settings/strategy-row";
import type {
	StrategyDraft,
	StrategyFormValues,
	StrategyKind,
} from "@/features/strategy-settings/types";
import { useCanonicalRules } from "@/features/strategy-settings/use-canonical-rules";
import { useStrategyForm } from "@/features/strategy-settings/use-strategy-form";
import {
	newStrategyDraft,
	strategyDraft,
	strategyErrorMessage,
	strategyFormDirty,
	strategyFormValues,
	strategyInput,
	strategyKind,
} from "@/features/strategy-settings/utils";

const selectStrategyCatalog = (response: getStrategyCatalogResponseSuccess) =>
	response.data;

const selectStrategiesOfKind =
	(kind: StrategyKind) => (response: listStrategiesResponseSuccess) =>
		response.data.items.filter((strategy) => strategyKind(strategy) === kind);

// One stable select per kind, so the list is filtered again only when the
// strategies change, not on every render.
const selectStrategies = {
	signal: selectStrategiesOfKind("signal"),
	strategy: selectStrategiesOfKind("strategy"),
} satisfies Record<StrategyKind, ReturnType<typeof selectStrategiesOfKind>>;

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
	// A strategy waiting to replace an open form with unsaved changes.
	const [pending, setPending] = useState<Omit<StrategyDraft, "revision">>();
	const revision = useRef(0);
	const [removing, setRemoving] = useState<Strategy>();
	const strategies = useListStrategies({
		query: {
			retry: false,
			select: selectStrategies[kind],
		},
	});
	const catalog = useGetStrategyCatalog({
		query: {
			retry: false,
			select: selectStrategyCatalog,
		},
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

	const openingMutation = useCanonicalRules();

	const submit = (values: StrategyFormValues) => {
		if (!draft) return;
		const input = strategyInput(values, draft.signal);
		if (!input) return;
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
	// The fields of the open draft, reset to its values when it opens. This
	// page subscribes to none of them, so edits never render it.
	const form = useStrategyForm(draft?.values, submit);

	if (strategies.isPending || catalog.isPending) {
		return <Loader aria-label={`Loading ${nouns.many}`} />;
	}
	if (strategies.isError || catalog.isError) {
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
		openingMutation.isPending ||
		isSaving;
	// Rules need a period and something to read on it.
	const empty =
		catalog.data.intervals.length === 0 ||
		(catalog.data.candle_fields.length === 0 &&
			catalog.data.indicators.length === 0);
	const show = (next: Omit<StrategyDraft, "revision">) => {
		revision.current += 1;
		form.reset(next.values);
		setDraft({ ...next, revision: revision.current });
	};
	// Beside the list, opening another strategy would silently drop the
	// changes of the open one, so it asks first.
	const open = (next: Omit<StrategyDraft, "revision">) => {
		if (
			draft &&
			strategyFormDirty(
				draft.values,
				strategyFormValues(form.state.values),
				draft.signal,
			)
		) {
			setPending(next);
		} else {
			show(next);
		}
	};
	const list = (
		<Stack gap="md">
			{strategies.data.length === 0 ? (
				<Text c="dimmed" size="sm">
					{`No ${nouns.many} yet.`}
				</Text>
			) : null}
			{empty ? (
				<Text c="dimmed" size="sm">
					Add indicators first.
				</Text>
			) : null}
			{/* Full width on phones and in the sidebar. */}
			<Group grow>
				<Button disabled={empty} onClick={() => open(newStrategyDraft(kind))}>
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
								openingMutation.mutate(strategy, {
									onError: (error) =>
										notifications.show({
											color: "red",
											message: strategyErrorMessage(error),
											title: `${nouns.name} could not be opened`,
										}),
									onSuccess: (rules) => open(strategyDraft(strategy, rules)),
								})
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
					catalog={catalog.data}
					form={form}
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
							catalog={catalog.data}
							form={form}
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
