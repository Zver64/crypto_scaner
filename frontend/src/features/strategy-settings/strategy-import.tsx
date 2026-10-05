import {
	Alert,
	Button,
	Group,
	List,
	Modal,
	Text,
	Textarea,
} from "@mantine/core";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
	useCreateScannerIndicatorBatch,
	useValidateStrategy,
} from "@/api/generated/api";
import type { StrategyMissingIndicator } from "@/api/generated/models";
import { invalidateScannerIndicatorQueries } from "@/features/scanner-settings/query-cache";
import { mutationErrorMessage } from "@/features/scanner-settings/utils";
import type { StrategyQuery } from "@/features/strategy-settings/types";
import { importedStrategyQuery } from "@/features/strategy-settings/utils";

interface StrategyImportProps {
	onClose(): void;
	onImport(query: StrategyQuery): void;
	opened: boolean;
}

// An expression whose only problem is indicators that are not configured yet,
// waiting for the administrator to add them.
interface PendingImport {
	missing: StrategyMissingIndicator[];
	query: StrategyQuery;
}

// Imports a whole expression into the builder once the backend finds no
// problem in it and the builder can show all of it. Indicators the expression
// reads but nobody configured are added first, once the administrator agrees.
export function StrategyImport({
	onClose,
	onImport,
	opened,
}: StrategyImportProps) {
	const queryClient = useQueryClient();
	const [expression, setExpression] = useState("");
	const [problems, setProblems] = useState<string[]>([]);
	const [pending, setPending] = useState<PendingImport>();
	const validation = useValidateStrategy();
	// The builder shows the new indicators once the variables are current.
	const addition = useCreateScannerIndicatorBatch({
		mutation: {
			onSuccess: () => invalidateScannerIndicatorQueries(queryClient),
		},
	});
	const busy = validation.isPending || addition.isPending;
	const finish = () => {
		setExpression("");
		setProblems([]);
		setPending(undefined);
		onClose();
	};
	// A running check or addition would import after the dialog closed.
	const close = () => {
		if (!busy) finish();
	};
	const check = () =>
		validation.mutate(
			{ data: { expression } },
			{
				onError: () => setProblems(["The expression could not be checked."]),
				onSuccess: (response) => {
					if (response.data.errors.length > 0) {
						setProblems(response.data.errors);
						return;
					}
					const query = importedStrategyQuery(expression);
					if (!query) {
						setProblems([
							"The builder cannot show part of this expression, such as a unary minus.",
						]);
						return;
					}
					if (response.data.missing_indicators.length > 0) {
						setPending({ missing: response.data.missing_indicators, query });
						return;
					}
					onImport(query);
					finish();
				},
			},
		);
	const add = (current: PendingImport) =>
		addition.mutate(
			{
				data: {
					items: current.missing.map(({ interval, parameters, type }) => ({
						interval,
						parameters,
						type,
					})),
				},
			},
			{
				onError: (error) => {
					setPending(undefined);
					setProblems([mutationErrorMessage(error)]);
				},
				onSuccess: () => {
					onImport(current.query);
					finish();
				},
			},
		);
	return (
		<Modal centered onClose={close} opened={opened} title="Import strategy">
			<Textarea
				autosize
				data-autofocus
				disabled={busy || pending !== undefined}
				label="Expression"
				minRows={4}
				onChange={(event) => {
					setExpression(event.currentTarget.value);
					setProblems([]);
				}}
				placeholder="h_rsi < 30 && crosses_above(h_close, h_sma_20)"
				styles={(theme) => ({
					input: { fontFamily: theme.fontFamilyMonospace },
				})}
				value={expression}
			/>
			{problems.length > 0 ? (
				<Alert
					color="red"
					mt="md"
					title="The expression cannot be imported"
					variant="light"
				>
					<List size="sm" spacing={4}>
						{problems.map((problem) => (
							<List.Item key={problem}>{problem}</List.Item>
						))}
					</List>
				</Alert>
			) : null}
			{pending ? (
				<Alert
					color="yellow"
					mt="md"
					title={`Add ${pending.missing.length} missing ${pending.missing.length === 1 ? "indicator" : "indicators"}?`}
					variant="light"
				>
					<Text size="sm">
						The expression reads indicators that are not configured. They are
						added without table columns or chart lines.
					</Text>
					<List mt="xs" size="sm" spacing={4}>
						{pending.missing.map((indicator) => (
							<List.Item key={indicator.title}>{indicator.title}</List.Item>
						))}
					</List>
				</Alert>
			) : null}
			<Group justify="flex-end" mt="md">
				<Button
					disabled={busy}
					onClick={pending ? () => setPending(undefined) : close}
					variant="default"
				>
					{pending ? "No" : "Cancel"}
				</Button>
				{pending ? (
					<Button loading={addition.isPending} onClick={() => add(pending)}>
						Add and import
					</Button>
				) : (
					<Button
						disabled={expression.trim() === ""}
						loading={validation.isPending}
						onClick={check}
					>
						Check and import
					</Button>
				)}
			</Group>
		</Modal>
	);
}
