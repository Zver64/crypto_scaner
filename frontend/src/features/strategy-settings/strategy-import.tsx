import {
	Alert,
	Button,
	Group,
	List,
	Modal,
	Text,
	Textarea,
} from "@mantine/core";
import { useState } from "react";
import { useValidateStrategy } from "@/api/generated/api";
import type { StrategyQuery } from "@/features/strategy-settings/types";
import { importedStrategyQuery } from "@/features/strategy-settings/utils";

interface StrategyImportProps {
	onClose(): void;
	onImport(query: StrategyQuery): void;
	opened: boolean;
	// Whether the builder already has conditions the import replaces.
	replaces: boolean;
}

// Imports a whole expression into the builder once the backend finds no
// problem in it and the builder can show all of it.
export function StrategyImport({
	onClose,
	onImport,
	opened,
	replaces,
}: StrategyImportProps) {
	const [expression, setExpression] = useState("");
	const [problems, setProblems] = useState<string[]>([]);
	const validation = useValidateStrategy();
	const finish = () => {
		setExpression("");
		setProblems([]);
		onClose();
	};
	// A running check would import after the dialog closed.
	const close = () => {
		if (!validation.isPending) finish();
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
					onImport(query);
					finish();
				},
			},
		);
	return (
		<Modal centered onClose={close} opened={opened} title="Import strategy">
			<Textarea
				autosize
				data-autofocus
				disabled={validation.isPending}
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
			{replaces ? (
				<Text c="dimmed" mt="xs" size="xs">
					The imported expression replaces the current conditions.
				</Text>
			) : null}
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
			<Group justify="flex-end" mt="md">
				<Button
					disabled={validation.isPending}
					onClick={close}
					variant="default"
				>
					Cancel
				</Button>
				<Button
					disabled={expression.trim() === ""}
					loading={validation.isPending}
					onClick={check}
				>
					Check and import
				</Button>
			</Group>
		</Modal>
	);
}
