import { Alert, Button, Group, List, Modal } from "@mantine/core";
import { useSelector } from "@tanstack/react-form";
import { useState } from "react";

import { useValidateStrategy } from "@/api/generated/api";
import type { StrategyIndicatorCall } from "@/api/generated/models";
import { useAppForm } from "@/components/form";
import type { StrategyQuery } from "@/features/strategy-settings/types";
import { importedStrategyQuery } from "@/features/strategy-settings/utils";

interface StrategyImportProps {
	// Whether the expression is an exit rule, which may read the position
	// variables.
	exit: boolean;
	onClose(): void;
	// The query of the canonical expression and the indicators it reads.
	onImport(query: StrategyQuery, indicators: StrategyIndicatorCall[]): void;
	opened: boolean;
}

// Imports a whole expression into the builder once the backend finds no
// problem in it and the builder can show all of its canonical form.
export function StrategyImport({
	exit,
	onClose,
	onImport,
	opened,
}: StrategyImportProps) {
	const form = useAppForm({ defaultValues: { expression: "" } });
	// The import button follows the text.
	const expression = useSelector(
		form.store,
		(state) => state.values.expression,
	);
	const [problems, setProblems] = useState<string[]>([]);
	const validation = useValidateStrategy();
	const finish = () => {
		form.reset();
		setProblems([]);
		onClose();
	};
	// A running check would import after the dialog closed.
	const close = () => {
		if (!validation.isPending) finish();
	};
	const check = () =>
		validation.mutate(
			{ data: { expression, kind: exit ? "exit" : "entry" } },
			{
				onError: () => setProblems(["The expression could not be checked."]),
				onSuccess: ({ data }) => {
					if (data.errors.length > 0) {
						setProblems(data.errors);
						return;
					}
					const query = importedStrategyQuery(data.expression);
					if (!query) {
						setProblems([
							"The builder cannot show part of this expression, such as a unary minus.",
						]);
						return;
					}
					onImport(query, data.indicators);
					finish();
				},
			},
		);
	return (
		<Modal centered onClose={close} opened={opened} title="Import strategy">
			<form.AppField
				listeners={{ onChange: () => setProblems([]) }}
				name="expression"
			>
				{(field) => (
					<field.TextareaField
						autosize
						data-autofocus
						disabled={validation.isPending}
						label="Expression"
						minRows={4}
						placeholder='rsi("1h") < 30 && CrossesAbove(close("1h"), sma("1h", 20))'
						styles={(theme) => ({
							input: { fontFamily: theme.fontFamilyMonospace },
						})}
					/>
				)}
			</form.AppField>
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
